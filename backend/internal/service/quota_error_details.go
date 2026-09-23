package service

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	pkgerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func platformQuotaError(err error, reset time.Time, limit, used float64) error {
	return quotaErrorDetails(err, limit, used, &reset, time.Time{}, time.Now())
}

func apiKeyWindowQuotaError(err error, start *time.Time, period time.Duration, limit, used float64) error {
	var reset *time.Time
	if start != nil {
		next := start.Add(period)
		reset = &next
	}
	return quotaErrorDetails(err, limit, used, reset, time.Time{}, time.Now())
}

// quotaErrorDetails uses the enforcement window, never an estimated reset date.
func quotaErrorDetails(err error, limit, used float64, reset *time.Time, expires time.Time, now time.Time) error {
	e := pkgerrors.FromError(err).WithMetadata(map[string]string{
		"limit_usd": strconv.FormatFloat(limit, 'f', -1, 64),
		"used_usd":  strconv.FormatFloat(used, 'f', -1, 64),
	})
	label := map[string]string{
		"DAILY_LIMIT_EXCEEDED": "套餐日限额", "WEEKLY_LIMIT_EXCEEDED": "套餐周限额",
		"MONTHLY_LIMIT_EXCEEDED": "套餐月限额", "TOTAL_LIMIT_EXCEEDED": "套餐总限额",
		"USER_PLATFORM_DAILY_QUOTA_EXHAUSTED":   "当前平台日限额",
		"USER_PLATFORM_WEEKLY_QUOTA_EXHAUSTED":  "当前平台周限额",
		"USER_PLATFORM_MONTHLY_QUOTA_EXHAUSTED": "当前平台月限额",
		"API_KEY_RATE_5H_EXCEEDED":              "API Key 5小时限额",
		"API_KEY_RATE_1D_EXCEEDED":              "API Key 日限额",
		"API_KEY_RATE_7D_EXCEEDED":              "API Key 7天限额",
	}[e.Reason]
	message := fmt.Sprintf("%s $%s 已用尽（已用 $%s）", label, e.Metadata["limit_usd"], e.Metadata["used_usd"])
	english := canonicalEnglishClientErrorMessages[e.Message]
	if english == "" {
		english = e.Message
	}
	if strings.HasPrefix(e.Reason, "API_KEY_RATE_") {
		english = "The API key " + strings.TrimPrefix(strings.TrimSuffix(e.Reason, "_EXCEEDED"), "API_KEY_RATE_") + " quota has been exhausted."
	}
	if first, _, ok := strings.Cut(english, ". "); ok {
		english = first + "."
	}
	english = fmt.Sprintf("%s Limit: USD %s; used: USD %s.", english, e.Metadata["limit_usd"], e.Metadata["used_usd"])
	switch {
	case e.Reason == "TOTAL_LIMIT_EXCEEDED":
		message += "，总额度不会自动恢复，请续费或更换套餐。"
		english += " This total quota does not reset automatically."
	case reset != nil && !expires.IsZero() && !reset.Before(expires):
		message += "，套餐将在额度重置前到期，请续费或更换套餐。"
		english += " The subscription expires before the next reset. Renew or change your plan."
	case reset != nil && reset.After(now):
		e.Metadata["window_resets_at"] = reset.UTC().Format(time.RFC3339)
		minutes := int64(math.Ceil(reset.Sub(now).Minutes()))
		message += fmt.Sprintf("，还有 %d 天 %d 小时 %d 分钟恢复（%s）。", minutes/1440, minutes%1440/60, minutes%60, reset.Format(time.RFC3339))
		english += fmt.Sprintf(" Resets in %d days %d hours %d minutes (%s).", minutes/1440, minutes%1440/60, minutes%60, reset.Format(time.RFC3339))
	default:
		message += "，暂无法确定恢复时间，请联系管理员。"
		english += " Reset time is unavailable; contact the administrator."
	}
	e.Message = english + "\n" + message
	return e
}

func subscriptionLimitError(err error, sub *UserSubscription, group *Group) error {
	if sub == nil || group == nil {
		return err
	}
	var limit *float64
	var used float64
	var reset *time.Time
	switch pkgerrors.Reason(err) {
	case "DAILY_LIMIT_EXCEEDED":
		limit, used, reset = group.DailyLimitUSD, sub.DailyUsageUSD, sub.DailyResetTime()
	case "WEEKLY_LIMIT_EXCEEDED":
		limit, used, reset = group.WeeklyLimitUSD, sub.WeeklyUsageUSD, sub.WeeklyResetTime()
	case "MONTHLY_LIMIT_EXCEEDED":
		limit, used, reset = group.MonthlyLimitUSD, sub.MonthlyUsageUSD, sub.MonthlyResetTime()
	case "TOTAL_LIMIT_EXCEEDED":
		limit, used = group.TotalLimitUSD, sub.TotalUsageUSD
	}
	if limit == nil {
		return err
	}
	return quotaErrorDetails(err, *limit, used, reset, sub.ExpiresAt, time.Now())
}
