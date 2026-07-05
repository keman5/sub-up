package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/redis/go-redis/v9"
	"github.com/tidwall/gjson"
)

const (
	troubleshootingMaxInputRunes       = 4000
	troubleshootingAIRequestTimeout    = 30 * time.Second
	troubleshootingMaxAIAccounts       = 3
	troubleshootingShortWindowLimit    = 10
	troubleshootingDailyLimit          = 50
	troubleshootingShortWindowDuration = 5 * time.Minute
	troubleshootingDailyDuration       = 24 * time.Hour
)

const (
	troubleshootingLocaleChinese = "zh"
	troubleshootingLocaleEnglish = "en"
)

type TroubleshootingAnalyzeInput struct {
	UserID  int64
	Message string
	Locale  string
}

type TroubleshootingAnalysis struct {
	Answer      string                     `json:"answer"`
	Source      string                     `json:"source"`
	NeedsAdmin  bool                       `json:"needs_admin"`
	AIAttempted bool                       `json:"ai_attempted"`
	AIAvailable bool                       `json:"ai_available"`
	AIAttempts  int                        `json:"ai_attempts"`
	Limit       *TroubleshootingLimitState `json:"limit,omitempty"`
}

type TroubleshootingLimitState struct {
	ShortWindowRemaining int `json:"short_window_remaining"`
	DailyRemaining       int `json:"daily_remaining"`
}

type TroubleshootingAdminNotifyInput struct {
	UserID    int64
	Message   string
	Diagnosis string
	Locale    string
}

type TroubleshootingAdminNotifyResult struct {
	Message string `json:"message"`
}

type TroubleshootingRateLimiter interface {
	Allow(ctx context.Context, userID int64) (*TroubleshootingLimitState, error)
}

type TroubleshootingAIClient interface {
	Diagnose(ctx context.Context, report string, localHint string, locale string) (answer string, attempts int, err error)
}

type TroubleshootingEvidenceProvider interface {
	Collect(ctx context.Context, report string, locale string) (*TroubleshootingEvidence, error)
}

type TroubleshootingEvidence struct {
	Confirmed           bool
	Reason              string
	NeedsAdmin          bool
	Request             *TroubleshootingEvidenceRequest
	CurrentAvailable    bool
	TotalAccounts       int
	SchedulableAccounts int
	UserAction          string
	AdminAction         string
}

type TroubleshootingEvidenceRequest struct {
	RequestID  string
	StatusCode int
	Phase      string
	Model      string
	Message    string
	CreatedAt  time.Time
}

type SystemTroubleshootingEvidenceProvider struct {
	accountRepo AccountRepository
	opsRepo     OpsRepository
}

func NewSystemTroubleshootingEvidenceProvider(accountRepo AccountRepository, opsRepo OpsRepository) *SystemTroubleshootingEvidenceProvider {
	return &SystemTroubleshootingEvidenceProvider{
		accountRepo: accountRepo,
		opsRepo:     opsRepo,
	}
}

type TroubleshootingAssistantService struct {
	ai                       TroubleshootingAIClient
	limiter                  TroubleshootingRateLimiter
	evidence                 TroubleshootingEvidenceProvider
	settingRepo              SettingRepository
	notificationEmailService *NotificationEmailService
	nowFunc                  func() time.Time
}

func NewTroubleshootingAssistantService(ai TroubleshootingAIClient, limiter TroubleshootingRateLimiter, evidence TroubleshootingEvidenceProvider) *TroubleshootingAssistantService {
	return &TroubleshootingAssistantService{
		ai:       ai,
		limiter:  limiter,
		evidence: evidence,
		nowFunc:  time.Now,
	}
}

func ProvideTroubleshootingAssistantService(
	ai TroubleshootingAIClient,
	limiter TroubleshootingRateLimiter,
	evidence TroubleshootingEvidenceProvider,
	settingRepo SettingRepository,
	notificationEmailService *NotificationEmailService,
) *TroubleshootingAssistantService {
	svc := NewTroubleshootingAssistantService(ai, limiter, evidence)
	svc.settingRepo = settingRepo
	svc.notificationEmailService = notificationEmailService
	return svc
}

func (s *TroubleshootingAssistantService) Analyze(ctx context.Context, input TroubleshootingAnalyzeInput) (*TroubleshootingAnalysis, error) {
	locale := normalizeTroubleshootingLocale(input.Locale)
	report := truncateRunes(strings.TrimSpace(input.Message), troubleshootingMaxInputRunes)
	if report == "" {
		return nil, infraerrors.BadRequest("TROUBLESHOOTING_EMPTY_REPORT", "请粘贴需要排查的报错信息")
	}
	if !looksLikeTroubleshootingReport(report) {
		return nil, infraerrors.BadRequest("TROUBLESHOOTING_ONLY", "这里只能排查请求失败原因，请粘贴报错信息、状态码、接口 URL 或 request id")
	}

	limitState, err := s.allow(ctx, input.UserID)
	if err != nil {
		return nil, err
	}

	evidence := s.collectEvidence(ctx, report, locale)
	local := buildTroubleshootingLocalDiagnosis(report, evidence, locale)
	result := &TroubleshootingAnalysis{
		Answer:     local.Answer,
		Source:     "rules",
		NeedsAdmin: local.NeedsAdmin,
		Limit:      limitState,
	}
	if local.Confirmed {
		return result, nil
	}
	if s.ai == nil {
		result.Answer = appendAIDisabledNoticeForLocale(result.Answer, result.NeedsAdmin, locale)
		return result, nil
	}

	aiCtx, cancel := context.WithTimeout(ctx, troubleshootingAIRequestTimeout)
	defer cancel()
	aiAnswer, attempts, err := s.ai.Diagnose(aiCtx, report, local.Answer, locale)
	result.AIAttempted = true
	result.AIAttempts = attempts
	if err != nil || strings.TrimSpace(aiAnswer) == "" {
		result.Answer = appendAIUnavailableNoticeForLocale(local.Answer, local.NeedsAdmin, locale)
		return result, nil
	}

	result.Answer = normalizeTroubleshootingAnswer(aiAnswer)
	result.Source = "ai"
	result.AIAvailable = true
	if local.NeedsAdmin || aiAnswerNeedsAdmin(result.Answer) {
		result.NeedsAdmin = true
	}
	if !result.NeedsAdmin {
		result.Answer = stripTroubleshootingAdminSections(result.Answer)
	}
	return result, nil
}

func (s *TroubleshootingAssistantService) NotifyAdmin(ctx context.Context, input TroubleshootingAdminNotifyInput) (*TroubleshootingAdminNotifyResult, error) {
	locale := normalizeTroubleshootingLocale(input.Locale)
	report := truncateRunes(strings.TrimSpace(input.Message), troubleshootingMaxInputRunes)
	diagnosis := truncateRunes(strings.TrimSpace(input.Diagnosis), troubleshootingMaxInputRunes)
	if report == "" || diagnosis == "" {
		return nil, infraerrors.BadRequest("TROUBLESHOOTING_NOTIFY_EMPTY", "缺少需要通知管理员的排查内容")
	}
	if !looksLikeTroubleshootingReport(report) {
		return nil, infraerrors.BadRequest("TROUBLESHOOTING_ONLY", "这里只能排查请求失败原因，请粘贴报错信息、状态码、接口 URL 或 request id")
	}
	if s == nil {
		return nil, infraerrors.ServiceUnavailable("TROUBLESHOOTING_NOTIFY_UNAVAILABLE", "管理员通知暂不可用")
	}
	if _, err := s.allow(ctx, input.UserID); err != nil {
		return nil, err
	}
	if s.settingRepo == nil || s.notificationEmailService == nil {
		return nil, infraerrors.ServiceUnavailable("TROUBLESHOOTING_NOTIFY_UNAVAILABLE", "管理员通知暂不可用")
	}
	recipients := s.troubleshootingNotifyEmails(ctx)
	if len(recipients) == 0 {
		return nil, infraerrors.ServiceUnavailable("TROUBLESHOOTING_NOTIFY_EMAILS_EMPTY", "管理员通知邮箱未配置")
	}

	now := s.now()
	bucket := strconv.FormatInt(now.UTC().Unix()/600, 10)
	sourceID := troubleshootingNotifySourceID(report)
	alreadySent, err := s.troubleshootingAdminNotifyAlreadySent(ctx, recipients, sourceID, bucket)
	if err != nil {
		return nil, err
	}
	if alreadySent {
		return &TroubleshootingAdminNotifyResult{Message: troubleshootingAdminNotifyAlreadySentMessage(locale)}, nil
	}
	var lastErr error
	sent := 0
	for _, recipient := range recipients {
		err := s.notificationEmailService.Send(ctx, NotificationEmailSendInput{
			Event:          NotificationEmailEventTroubleshootingAdminNotify,
			Locale:         locale,
			RecipientEmail: recipient,
			RecipientName:  emailRecipientName(recipient),
			UserID:         input.UserID,
			SourceType:     "troubleshooting_admin_notify",
			SourceID:       sourceID,
			ReminderKey:    bucket,
			Variables: map[string]string{
				"user_id":    strconv.FormatInt(input.UserID, 10),
				"message":    report,
				"diagnosis":  diagnosis,
				"request_id": firstNonEmpty(extractTroubleshootingRequestID(report), "-"),
				"status_code": func() string {
					if status := parseTroubleshootingStatusCode(report); status > 0 {
						return strconv.Itoa(status)
					}
					return "-"
				}(),
				"reported_at": now.Format("2006-01-02 15:04:05"),
			},
		})
		if err != nil {
			lastErr = err
			continue
		}
		sent++
	}
	if sent == 0 && lastErr != nil {
		return nil, lastErr
	}
	return &TroubleshootingAdminNotifyResult{Message: troubleshootingAdminNotifySuccessMessage(locale)}, nil
}

func (s *TroubleshootingAssistantService) troubleshootingNotifyEmails(ctx context.Context) []string {
	if s == nil || s.settingRepo == nil {
		return nil
	}
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyAccountQuotaNotifyEmails)
	if err != nil || strings.TrimSpace(raw) == "" || strings.TrimSpace(raw) == "[]" {
		return nil
	}
	return filterVerifiedEmails(ParseNotifyEmails(raw))
}

func (s *TroubleshootingAssistantService) troubleshootingAdminNotifyAlreadySent(ctx context.Context, recipients []string, sourceID string, bucket string) (bool, error) {
	if s == nil || s.notificationEmailService == nil {
		return false, nil
	}
	for _, recipient := range recipients {
		sent, err := s.notificationEmailService.deliveryExists(
			ctx,
			notificationEmailDeliveryKey(NotificationEmailEventTroubleshootingAdminNotify, "troubleshooting_admin_notify", sourceID, recipient, bucket),
			legacyNotificationEmailDeliveryKey(NotificationEmailEventTroubleshootingAdminNotify, "troubleshooting_admin_notify", sourceID, recipient, bucket),
		)
		if err != nil {
			return false, err
		}
		if sent {
			return true, nil
		}
	}
	return false, nil
}

func troubleshootingNotifySourceID(report string) string {
	h := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(report))))
	return hex.EncodeToString(h[:16])
}

func troubleshootingAdminNotifySuccessMessage(locale string) string {
	if locale == troubleshootingLocaleEnglish {
		return "The administrator has been notified. Please wait 5 minutes and retry."
	}
	return "已通知管理员，请等待 5 分钟后重试。"
}

func troubleshootingAdminNotifyAlreadySentMessage(locale string) string {
	if locale == troubleshootingLocaleEnglish {
		return "The administrator has received this issue and is handling it. Please wait 5 minutes and retry."
	}
	return "管理员已收到，正在处理，请等待 5 分钟后重试。"
}

func (s *TroubleshootingAssistantService) now() time.Time {
	if s == nil || s.nowFunc == nil {
		return time.Now()
	}
	return s.nowFunc()
}

func (s *TroubleshootingAssistantService) collectEvidence(ctx context.Context, report string, locale string) *TroubleshootingEvidence {
	if s == nil || s.evidence == nil {
		return nil
	}
	evidence, err := s.evidence.Collect(ctx, report, locale)
	if err != nil {
		return nil
	}
	return evidence
}

func (p *SystemTroubleshootingEvidenceProvider) Collect(ctx context.Context, report string, locale string) (*TroubleshootingEvidence, error) {
	if p == nil {
		return nil, nil
	}
	locale = normalizeTroubleshootingLocale(locale)

	now := time.Now()
	requestID := extractTroubleshootingRequestID(report)
	statusCode := parseTroubleshootingStatusCode(report)

	if p.opsRepo != nil && requestID != "" {
		start := now.Add(-6 * time.Hour)
		end := now.Add(5 * time.Minute)
		items, _, err := p.opsRepo.ListRequestDetails(ctx, &OpsRequestDetailFilter{
			StartTime: &start,
			EndTime:   &end,
			Kind:      string(OpsRequestKindError),
			RequestID: requestID,
			Page:      1,
			PageSize:  1,
		})
		if err == nil && len(items) > 0 && items[0] != nil {
			return troubleshootingEvidenceFromRequestDetail(items[0], locale), nil
		}
	}

	accountEvidence := p.currentAccountPoolEvidence(ctx)
	if accountEvidence == nil {
		return nil, nil
	}
	if statusCode >= 500 || containsAny(strings.ToLower(report), "service unavailable", "bad gateway", "gateway timeout", "no available accounts") {
		if accountEvidence.SchedulableAccounts > 0 {
			accountEvidence.Confirmed = true
			accountEvidence.NeedsAdmin = false
			if requestID != "" {
				if locale == troubleshootingLocaleEnglish {
					accountEvidence.Reason = fmt.Sprintf("No failure record was found for request id %s, and the current OpenAI account pool has %d schedulable account(s). The issue may have recovered.", requestID, accountEvidence.SchedulableAccounts)
				} else {
					accountEvidence.Reason = fmt.Sprintf("系统未查到 request id %s 对应的失败记录，且当前 OpenAI 账号池已有 %d 个可用账号。该故障可能已恢复。", requestID, accountEvidence.SchedulableAccounts)
				}
			} else {
				if locale == troubleshootingLocaleEnglish {
					accountEvidence.Reason = fmt.Sprintf("No request id was provided for an exact lookup, and the current OpenAI account pool has %d schedulable account(s). The issue may have recovered.", accountEvidence.SchedulableAccounts)
				} else {
					accountEvidence.Reason = fmt.Sprintf("未提供可精确查询的 request id，且当前 OpenAI 账号池已有 %d 个可用账号。该故障可能已恢复。", accountEvidence.SchedulableAccounts)
				}
			}
			if locale == troubleshootingLocaleEnglish {
				accountEvidence.UserAction = "Retry the request. If it still fails, provide the new request id."
			} else {
				accountEvidence.UserAction = "请重新发起请求；如果仍失败，请提供新的 request id。"
			}
			accountEvidence.AdminAction = ""
			return accountEvidence, nil
		}

		accountEvidence.Confirmed = true
		accountEvidence.NeedsAdmin = true
		if locale == troubleshootingLocaleEnglish {
			accountEvidence.Reason = "The current OpenAI account pool has no schedulable accounts. An administrator needs to check account status, quota, or proxy connectivity."
			accountEvidence.UserAction = "This cannot be fixed by retrying right now. Send the error and request id to an administrator."
			accountEvidence.AdminAction = "Check OpenAI account status, temporary unschedulable reasons, quota windows, and proxy connectivity."
		} else {
			accountEvidence.Reason = "当前 OpenAI 账号池没有可用账号，请联系管理员处理账号、额度或代理链路。"
			accountEvidence.UserAction = "当前无法通过重试自行恢复，请把报错和 request id 发给管理员。"
			accountEvidence.AdminAction = "检查 OpenAI 账号状态、临时不可调度原因、额度窗口和代理链路。"
		}
		return accountEvidence, nil
	}

	return nil, nil
}

func (p *SystemTroubleshootingEvidenceProvider) currentAccountPoolEvidence(ctx context.Context) *TroubleshootingEvidence {
	if p == nil || p.accountRepo == nil {
		return nil
	}
	accounts, err := p.accountRepo.ListActive(ctx)
	if err != nil {
		return nil
	}
	evidence := &TroubleshootingEvidence{}
	for _, account := range accounts {
		if account.Platform != PlatformOpenAI {
			continue
		}
		if account.Type != AccountTypeOAuth && account.Type != AccountTypeAPIKey {
			continue
		}
		evidence.TotalAccounts++
		if account.IsSchedulable() {
			evidence.SchedulableAccounts++
		}
	}
	evidence.CurrentAvailable = evidence.SchedulableAccounts > 0
	return evidence
}

func troubleshootingEvidenceFromRequestDetail(item *OpsRequestDetail, locale string) *TroubleshootingEvidence {
	statusCode := 0
	if item.StatusCode != nil {
		statusCode = *item.StatusCode
	}
	message := strings.TrimSpace(item.Message)
	phase := strings.TrimSpace(item.Phase)

	if evidence := explicitTroubleshootingEvidenceFromMessage(message, statusCode, locale); evidence != nil {
		evidence.Request = &TroubleshootingEvidenceRequest{
			RequestID:  strings.TrimSpace(item.RequestID),
			StatusCode: statusCode,
			Phase:      phase,
			Model:      strings.TrimSpace(item.Model),
			Message:    message,
			CreatedAt:  item.CreatedAt,
		}
		return evidence
	}

	needsAdmin := statusCode >= 500 || statusCode == 429 || phase == "upstream" || phase == "network" || phase == "routing" || phase == "internal"
	reason := formatTroubleshootingLogReason(statusCode, phase, message, locale)
	userAction := "请稍后重试一次；如果仍失败，把 request id 发给管理员。"
	adminAction := "检查该 request id 对应的错误日志、上游账号、代理链路和请求模型。"
	if locale == troubleshootingLocaleEnglish {
		userAction = "Retry once later. If it still fails, send the request id to an administrator."
		adminAction = "Check the error log for this request id, upstream account, proxy path, and requested model."
	}
	if statusCode == 401 {
		needsAdmin = false
		if locale == troubleshootingLocaleEnglish {
			userAction = "Copy the API Key again and make sure there are no extra spaces."
			adminAction = "If the user confirms the key is correct, check key status, subscription status, and authentication logs."
		} else {
			userAction = "请重新复制 API Key，确认没有多余空格；如果仍失败再联系管理员。"
			adminAction = "如用户确认 Key 无误，检查 Key 状态、用户订阅和认证日志。"
		}
	} else if statusCode == 400 || statusCode == 404 {
		needsAdmin = false
		if locale == troubleshootingLocaleEnglish {
			userAction = "Check that base_url, API path, and model name match the console settings."
			adminAction = "If the user's configuration is correct, check model mappings and routing configuration."
		} else {
			userAction = "请检查 base_url、接口路径和模型名是否与控制台一致。"
			adminAction = "如用户配置无误，检查模型映射和路由配置。"
		}
	}

	return &TroubleshootingEvidence{
		Confirmed:  true,
		Reason:     reason,
		NeedsAdmin: needsAdmin,
		Request: &TroubleshootingEvidenceRequest{
			RequestID:  strings.TrimSpace(item.RequestID),
			StatusCode: statusCode,
			Phase:      phase,
			Model:      strings.TrimSpace(item.Model),
			Message:    message,
			CreatedAt:  item.CreatedAt,
		},
		UserAction:  userAction,
		AdminAction: adminAction,
	}
}

func formatTroubleshootingLogReason(statusCode int, phase string, message string, locale string) string {
	if locale == troubleshootingLocaleEnglish {
		statusText := "The request failed"
		if statusCode > 0 {
			statusText = fmt.Sprintf("The system found a failure record for this request: HTTP %d", statusCode)
		}
		phaseText := strings.TrimSpace(phase)
		if phaseText != "" {
			statusText += ", failure phase: " + phaseText
		}
		if message != "" {
			statusText += ", error: " + truncateRunes(message, 180)
		}
		statusText += "."
		return statusText
	}
	statusText := "请求失败"
	if statusCode > 0 {
		statusText = fmt.Sprintf("系统已查到该请求失败记录：HTTP %d", statusCode)
	}
	phaseText := strings.TrimSpace(phase)
	if phaseText != "" {
		statusText += "，失败阶段为 " + phaseText
	}
	if message != "" {
		statusText += "，错误为：" + truncateRunes(message, 180)
	}
	statusText += "。"
	return statusText
}

func explicitTroubleshootingEvidenceFromMessage(message string, statusCode int, locale string) *TroubleshootingEvidence {
	locale = normalizeTroubleshootingLocale(locale)
	lower := strings.ToLower(strings.TrimSpace(message))
	if lower == "" {
		return nil
	}

	switch {
	case isTroubleshootingSubscriptionUnavailable(lower):
		if locale == troubleshootingLocaleEnglish {
			return &TroubleshootingEvidence{
				Confirmed:   true,
				Reason:      "The user has no usable subscription for this group. The subscription has expired or is not assigned.",
				NeedsAdmin:  true,
				UserAction:  "Contact an administrator to renew or assign the subscription, then retry.",
				AdminAction: "Check the user's subscription for this API key group, including status, expiry time, and plan assignment.",
			}
		}
		return &TroubleshootingEvidence{
			Confirmed:   true,
			Reason:      "订阅已过期或未开通，当前用户没有该分组的可用订阅。请联系管理员续期。",
			NeedsAdmin:  true,
			UserAction:  "请联系管理员续期或重新分配套餐后再重试。",
			AdminAction: "检查该用户 API Key 所属分组的订阅状态、到期时间和套餐分配。",
		}
	case isTroubleshootingNoAvailableAccounts(lower):
		if locale == troubleshootingLocaleEnglish {
			return &TroubleshootingEvidence{
				Confirmed:   true,
				Reason:      "The system currently has no usable upstream account for this request.",
				NeedsAdmin:  true,
				UserAction:  "Contact an administrator and retry after the account pool is restored.",
				AdminAction: "Check upstream account status, model support, quota windows, temporary scheduling blocks, and proxy connectivity.",
			}
		}
		return &TroubleshootingEvidence{
			Confirmed:   true,
			Reason:      "系统当前没有可用账户，无法完成该请求。",
			NeedsAdmin:  true,
			UserAction:  "请联系管理员处理账号池后再重试。",
			AdminAction: "检查上游账号状态、模型支持、额度窗口、临时不可调度原因和代理链路。",
		}
	}

	return nil
}

func isTroubleshootingSubscriptionUnavailable(lower string) bool {
	return strings.Contains(lower, "no active subscription found for this group") ||
		strings.Contains(lower, "user does not have an active subscription for this group") ||
		strings.Contains(lower, "subscription_not_found") ||
		strings.Contains(lower, "subscription_required") ||
		strings.Contains(lower, "subscription is invalid or expired") ||
		strings.Contains(lower, "订阅已过期") ||
		strings.Contains(lower, "没有可用订阅") ||
		strings.Contains(lower, "无可用订阅")
}

func isTroubleshootingNoAvailableAccounts(lower string) bool {
	return strings.Contains(lower, "no available accounts") ||
		strings.Contains(lower, "account_select_failed") ||
		strings.Contains(lower, "没有可用账号") ||
		strings.Contains(lower, "没有可用账户") ||
		strings.Contains(lower, "无可用账号") ||
		strings.Contains(lower, "无可用账户")
}

var troubleshootingRequestIDPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\brequest[ _-]?id[:\s]+([a-z0-9._:-]{6,})`),
	regexp.MustCompile(`(?i)\breq[ _-]?id[:\s]+([a-z0-9._:-]{6,})`),
}

func extractTroubleshootingRequestID(report string) string {
	for _, pattern := range troubleshootingRequestIDPatterns {
		matches := pattern.FindStringSubmatch(report)
		if len(matches) > 1 {
			return strings.Trim(strings.TrimSpace(matches[1]), ".,;，。")
		}
	}
	return ""
}

func parseTroubleshootingStatusCode(report string) int {
	code := firstTroubleshootingStatusCode(strings.ToLower(report))
	if code == "" {
		return 0
	}
	n, _ := strconv.Atoi(code)
	return n
}

func (s *TroubleshootingAssistantService) allow(ctx context.Context, userID int64) (*TroubleshootingLimitState, error) {
	if userID <= 0 {
		return nil, infraerrors.Unauthorized("AUTH_REQUIRED", "User not authenticated")
	}
	if s.limiter == nil {
		return &TroubleshootingLimitState{
			ShortWindowRemaining: troubleshootingShortWindowLimit - 1,
			DailyRemaining:       troubleshootingDailyLimit - 1,
		}, nil
	}
	return s.limiter.Allow(ctx, userID)
}

type localTroubleshootingDiagnosis struct {
	Answer     string
	NeedsAdmin bool
	Confirmed  bool
}

func looksLikeTroubleshootingReport(report string) bool {
	lower := strings.ToLower(report)
	signals := []string{
		"http://", "https://", "/api/", "/v1/", "request id", "req id", "cf-ray",
		"error", "failed", "failure", "exception", "timeout", "unauthorized",
		"forbidden", "bad gateway", "service unavailable", "too many requests",
		"connection refused", "connect:", "status", "报错", "失败", "错误", "请求",
		"接口", "状态码", "上游", "额度", "限流", "模型", "无法", "异常",
	}
	for _, signal := range signals {
		if strings.Contains(lower, signal) {
			return true
		}
	}
	for _, code := range []string{"400", "401", "403", "404", "429", "500", "502", "503", "504", "529"} {
		if strings.Contains(lower, code) {
			return true
		}
	}
	return false
}

func buildTroubleshootingLocalDiagnosis(report string, evidence *TroubleshootingEvidence, locale string) localTroubleshootingDiagnosis {
	locale = normalizeTroubleshootingLocale(locale)
	if evidence != nil && evidence.Confirmed && strings.TrimSpace(evidence.Reason) != "" {
		return localTroubleshootingDiagnosis{
			Answer:     formatTroubleshootingEvidenceAnswer(evidence, locale),
			NeedsAdmin: evidence.NeedsAdmin,
			Confirmed:  true,
		}
	}
	if explicitEvidence := explicitTroubleshootingEvidenceFromMessage(report, parseTroubleshootingStatusCode(report), locale); explicitEvidence != nil {
		return localTroubleshootingDiagnosis{
			Answer:     formatTroubleshootingEvidenceAnswer(explicitEvidence, locale),
			NeedsAdmin: explicitEvidence.NeedsAdmin,
			Confirmed:  true,
		}
	}

	lower := strings.ToLower(report)
	cause := "请求失败信息不完整，暂时只能判断为接口调用异常。"
	needAdmin := false
	userChecks := []string{"确认使用的是当前环境提供的 API 端点。", "确认客户端填写的模型名、API Key 和网络代理没有写错。"}
	adminInfo := []string{"完整报错文本", "接口 URL", "状态码", "request id 或 cf-ray"}
	if locale == troubleshootingLocaleEnglish {
		cause = "The failure report is incomplete, so the system can only classify it as an API request failure."
		userChecks = []string{"Confirm that the API endpoint belongs to the current environment.", "Confirm that the model name, API Key, and network proxy are configured correctly."}
		adminInfo = []string{"Full error text", "API URL", "Status code", "request id or cf-ray"}
	}

	switch {
	case containsAny(lower, "502", "503", "504", "bad gateway", "service unavailable", "gateway timeout", "no available accounts"):
		if locale == troubleshootingLocaleEnglish {
			cause = "The upstream account pool, proxy path, or backend forwarding path is temporarily unavailable. This is commonly caused by insufficient schedulable accounts, upstream 5xx responses, proxy connection failures, or server overload."
		} else {
			cause = "上游账号池、代理链路或后端转发链路暂时不可用，常见于可用账号不足、上游 5xx、代理连接失败或服务过载。"
		}
		needAdmin = true
		if locale == troubleshootingLocaleEnglish {
			userChecks = append(userChecks, "Retry once later so temporary upstream instability is not mistaken for a configuration error.")
		} else {
			userChecks = append(userChecks, "稍后重试一次，避免把临时上游抖动误判为配置错误。")
		}
	case containsAny(lower, "429", "too many requests", "rate limit", "限流", "额度"):
		if locale == troubleshootingLocaleEnglish {
			cause = "The request hit a rate limit or quota window, such as account quota, subscription quota, RPM/concurrency limits, or upstream rate limits."
		} else {
			cause = "请求触发了限流或额度窗口，可能是账号额度、套餐额度、RPM/并发限制或上游 rate limit。"
		}
		needAdmin = true
		if locale == troubleshootingLocaleEnglish {
			userChecks = append(userChecks, "Reduce concurrency or wait for the rate-limit window to reset before retrying.")
		} else {
			userChecks = append(userChecks, "降低并发或等待限流窗口恢复后再试。")
		}
	case containsAny(lower, "401", "unauthorized", "invalid api key", "api_key_required", "token_invalidated", "token_revoked"):
		if locale == troubleshootingLocaleEnglish {
			cause = "Authentication failed. Common causes include an incorrect API Key, a disabled key, an expired session, or an invalid upstream OAuth token."
			userChecks = append(userChecks, "Copy the API Key again and make sure there are no extra spaces.")
		} else {
			cause = "鉴权失败，常见原因是 API Key 填错、Key 已停用、登录态过期，或上游 OAuth token 失效。"
			userChecks = append(userChecks, "重新复制 API Key，确认没有多余空格。")
		}
	case containsAny(lower, "403", "forbidden", "permission", "policy", "blocked"):
		if locale == troubleshootingLocaleEnglish {
			cause = "The request was blocked by permission, risk-control, or policy checks. It may relate to account permissions, model permissions, content moderation, or Cloudflare/upstream policy."
		} else {
			cause = "请求被权限、风控或策略拦截，可能与账号权限、模型权限、内容风控或 Cloudflare/上游策略有关。"
		}
		needAdmin = true
	case containsAny(lower, "connection refused", "connect:", "socks", "proxy", "timeout", "context canceled", "stream closed"):
		if locale == troubleshootingLocaleEnglish {
			cause = "A network or proxy path failed. Common causes include unavailable WARP/SOCKS/proxy services, upstream connection timeouts, or the client interrupting a streaming request."
		} else {
			cause = "网络或代理链路异常，常见于 WARP/SOCKS/代理服务不可用、上游连接超时或客户端中断流式请求。"
		}
		needAdmin = true
	case containsAny(lower, "400", "selected model", "model", "invalid_request", "not found"):
		if locale == troubleshootingLocaleEnglish {
			cause = "Request parameters or model mapping may not match. Common causes include an incorrect model name, mismatched endpoint path, or a message format rejected by the upstream service."
			userChecks = append(userChecks, "Compare the endpoint and model list in the console, and confirm the client's base_url and model name match.")
		} else {
			cause = "请求参数或模型映射可能不匹配，常见于模型名写错、端点路径不匹配、消息格式不符合上游要求。"
			userChecks = append(userChecks, "对照控制台里的端点和模型列表，确认客户端使用的 base_url 与模型名一致。")
		}
	}
	if status := firstTroubleshootingStatusCode(lower); status != "" {
		if locale == troubleshootingLocaleEnglish {
			cause = "Detected HTTP " + status + ". " + cause
		} else {
			cause = "识别到 HTTP " + status + "。 " + cause
		}
	}

	answer := formatTroubleshootingAnswer(cause, needAdmin, userChecks, adminInfo, locale)
	return localTroubleshootingDiagnosis{Answer: answer, NeedsAdmin: needAdmin}
}

func formatTroubleshootingEvidenceAnswer(evidence *TroubleshootingEvidence, locale string) string {
	userAction := strings.TrimSpace(evidence.UserAction)
	if userAction == "" {
		if locale == troubleshootingLocaleEnglish {
			userAction = "Retry once. If it still fails, provide the new request id."
		} else {
			userAction = "请重试一次；如果仍失败，请提供新的 request id。"
		}
	}
	adminAction := strings.TrimSpace(evidence.AdminAction)
	if adminAction == "" {
		if locale == troubleshootingLocaleEnglish {
			adminAction = "Full error text, API URL, status code, request id, or cf-ray."
		} else {
			adminAction = "完整报错文本、接口 URL、状态码、request id 或 cf-ray。"
		}
	}
	if locale == troubleshootingLocaleEnglish {
		lines := []string{
			"Diagnosis Result",
			strings.TrimSpace(evidence.Reason),
		}
		if evidence.Request != nil {
			requestFacts := formatTroubleshootingRequestEvidence(evidence.Request, locale)
			if requestFacts != "" {
				lines = append(lines, "", "System Record", requestFacts)
			}
		}
		lines = append(lines, "", "User Action", userAction)
		if evidence.NeedsAdmin {
			lines = append(lines,
				"",
				"Contact Administrator",
				"Administrator help is required.",
				"",
				"Information for Administrator",
				adminAction,
			)
		}
		return strings.Join(lines, "\n")
	}
	lines := []string{
		"排查结果",
		strings.TrimSpace(evidence.Reason),
	}
	if evidence.Request != nil {
		requestFacts := formatTroubleshootingRequestEvidence(evidence.Request, locale)
		if requestFacts != "" {
			lines = append(lines, "", "系统记录", requestFacts)
		}
	}
	lines = append(lines, "", "用户可执行操作", userAction)
	if evidence.NeedsAdmin {
		lines = append(lines,
			"",
			"需要联系管理员",
			"需要联系管理员处理或协助确认。",
			"",
			"建议提供给管理员的信息",
			adminAction,
		)
	}
	return strings.Join(lines, "\n")
}

func formatTroubleshootingRequestEvidence(request *TroubleshootingEvidenceRequest, locale string) string {
	if request == nil {
		return ""
	}
	var facts []string
	if request.RequestID != "" {
		facts = append(facts, "request id: "+request.RequestID)
	}
	if request.StatusCode > 0 {
		if locale == troubleshootingLocaleEnglish {
			facts = append(facts, "status code: "+strconv.Itoa(request.StatusCode))
		} else {
			facts = append(facts, "状态码: "+strconv.Itoa(request.StatusCode))
		}
	}
	if request.Phase != "" {
		if locale == troubleshootingLocaleEnglish {
			facts = append(facts, "phase: "+request.Phase)
		} else {
			facts = append(facts, "阶段: "+request.Phase)
		}
	}
	if request.Model != "" {
		if locale == troubleshootingLocaleEnglish {
			facts = append(facts, "model: "+request.Model)
		} else {
			facts = append(facts, "模型: "+request.Model)
		}
	}
	if request.Message != "" {
		if locale == troubleshootingLocaleEnglish {
			facts = append(facts, "error: "+truncateRunes(request.Message, 180))
		} else {
			facts = append(facts, "错误: "+truncateRunes(request.Message, 180))
		}
	}
	return strings.Join(facts, "\n")
}

func firstTroubleshootingStatusCode(report string) string {
	for _, code := range []string{"400", "401", "403", "404", "429", "500", "502", "503", "504", "529"} {
		if strings.Contains(report, code) {
			return code
		}
	}
	return ""
}

func formatTroubleshootingAnswer(cause string, needsAdmin bool, userChecks []string, adminInfo []string, locale string) string {
	if locale == troubleshootingLocaleEnglish {
		if !needsAdmin {
			return fmt.Sprintf("Possible Cause\n%s\n\nUser Checks\n%s",
				cause,
				formatNumberedList(userChecks),
			)
		}
		return fmt.Sprintf("Possible Cause\n%s\n\nContact Administrator\nAdministrator help is required.\n\nUser Checks\n%s\n\nInformation for Administrator\n%s",
			cause,
			formatNumberedList(userChecks),
			formatNumberedList(adminInfo),
		)
	}
	if !needsAdmin {
		return fmt.Sprintf("可能原因\n%s\n\n用户可自行检查项\n%s",
			cause,
			formatNumberedList(userChecks),
		)
	}
	return fmt.Sprintf("可能原因\n%s\n\n需要联系管理员\n需要联系管理员处理或协助确认。\n\n用户可自行检查项\n%s\n\n建议提供给管理员的信息\n%s",
		cause,
		formatNumberedList(userChecks),
		formatNumberedList(adminInfo),
	)
}

func formatNumberedList(items []string) string {
	var b strings.Builder
	for i, item := range items {
		b.WriteString(fmt.Sprintf("%d. %s", i+1, item))
		if i < len(items)-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func appendAIUnavailableNotice(answer string, needsAdmin bool) string {
	if needsAdmin {
		return strings.TrimSpace(answer) + "\n\nAI 诊断暂不可用，当前可用账号无法完成排查请求。请联系管理员，并附上上面的错误信息。"
	}
	return strings.TrimSpace(answer) + "\n\nAI 诊断暂不可用，已按本地规则给出结果，请按上面的检查项处理。"
}

func appendAIDisabledNotice(answer string, needsAdmin bool) string {
	if needsAdmin {
		return strings.TrimSpace(answer) + "\n\nAI 诊断未启用，请按以上规则诊断结果处理；如仍失败请联系管理员。"
	}
	return strings.TrimSpace(answer) + "\n\nAI 诊断未启用，请按以上规则诊断结果处理。"
}

func appendAIUnavailableNoticeForLocale(answer string, needsAdmin bool, locale string) string {
	if locale != troubleshootingLocaleEnglish {
		return appendAIUnavailableNotice(answer, needsAdmin)
	}
	if needsAdmin {
		return strings.TrimSpace(answer) + "\n\nAI diagnosis is temporarily unavailable because no usable account could complete the diagnostic request. Contact an administrator with the error information above."
	}
	return strings.TrimSpace(answer) + "\n\nAI diagnosis is temporarily unavailable. The result above was produced by local rules; follow the listed checks."
}

func appendAIDisabledNoticeForLocale(answer string, needsAdmin bool, locale string) string {
	if locale != troubleshootingLocaleEnglish {
		return appendAIDisabledNotice(answer, needsAdmin)
	}
	if needsAdmin {
		return strings.TrimSpace(answer) + "\n\nAI diagnosis is disabled. Follow the rule-based result above; contact an administrator if the issue continues."
	}
	return strings.TrimSpace(answer) + "\n\nAI diagnosis is disabled. Follow the rule-based result above."
}

func normalizeTroubleshootingAnswer(answer string) string {
	return truncateRunes(strings.TrimSpace(answer), 1600)
}

func stripTroubleshootingAdminSections(answer string) string {
	lines := strings.Split(strings.TrimSpace(answer), "\n")
	out := make([]string, 0, len(lines))
	skipAdminBlock := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if isTroubleshootingAdminHeading(trimmed) {
			skipAdminBlock = !strings.Contains(trimmed, ":") && !strings.Contains(trimmed, "：")
			continue
		}
		if skipAdminBlock {
			if isTroubleshootingNonAdminHeading(trimmed) {
				skipAdminBlock = false
			} else {
				continue
			}
		}
		out = append(out, line)
	}
	return normalizeTroubleshootingAnswer(strings.Join(trimEmptyTroubleshootingLines(out), "\n"))
}

func isTroubleshootingAdminHeading(line string) bool {
	normalized := strings.TrimSpace(strings.TrimRight(line, ":："))
	return normalized == "是否需要联系管理员" ||
		normalized == "需要联系管理员" ||
		normalized == "建议提供给管理员的信息" ||
		normalized == "Contact Administrator" ||
		normalized == "Information for Administrator" ||
		strings.HasPrefix(line, "是否需要联系管理员:") ||
		strings.HasPrefix(line, "是否需要联系管理员：") ||
		strings.HasPrefix(line, "需要联系管理员:") ||
		strings.HasPrefix(line, "需要联系管理员：") ||
		strings.HasPrefix(line, "建议提供给管理员的信息:") ||
		strings.HasPrefix(line, "建议提供给管理员的信息：") ||
		strings.HasPrefix(line, "Contact Administrator:") ||
		strings.HasPrefix(line, "Information for Administrator:")
}

func isTroubleshootingNonAdminHeading(line string) bool {
	normalized := strings.TrimSpace(strings.TrimRight(line, ":："))
	return normalized == "排查结果" ||
		normalized == "可能原因" ||
		normalized == "系统记录" ||
		normalized == "用户可执行操作" ||
		normalized == "用户可自行检查项" ||
		normalized == "Diagnosis Result" ||
		normalized == "Possible Cause" ||
		normalized == "System Record" ||
		normalized == "User Action" ||
		normalized == "User Checks" ||
		strings.HasPrefix(line, "排查结果:") ||
		strings.HasPrefix(line, "排查结果：") ||
		strings.HasPrefix(line, "可能原因:") ||
		strings.HasPrefix(line, "可能原因：") ||
		strings.HasPrefix(line, "系统记录:") ||
		strings.HasPrefix(line, "系统记录：") ||
		strings.HasPrefix(line, "用户可执行操作:") ||
		strings.HasPrefix(line, "用户可执行操作：") ||
		strings.HasPrefix(line, "用户可自行检查项:") ||
		strings.HasPrefix(line, "用户可自行检查项：") ||
		strings.HasPrefix(line, "Diagnosis Result:") ||
		strings.HasPrefix(line, "Possible Cause:") ||
		strings.HasPrefix(line, "System Record:") ||
		strings.HasPrefix(line, "User Action:") ||
		strings.HasPrefix(line, "User Checks:")
}

func trimEmptyTroubleshootingLines(lines []string) []string {
	out := make([]string, 0, len(lines))
	blank := false
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			if blank || len(out) == 0 {
				continue
			}
			blank = true
			out = append(out, "")
			continue
		}
		blank = false
		out = append(out, line)
	}
	for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
		out = out[:len(out)-1]
	}
	return out
}

func aiAnswerNeedsAdmin(answer string) bool {
	normalized := strings.ReplaceAll(strings.TrimSpace(answer), " ", "")
	if strings.Contains(normalized, "不需要联系管理员") || strings.Contains(normalized, "暂不需要联系管理员") {
		return false
	}
	lower := strings.ToLower(normalized)
	if strings.Contains(lower, "notrequired") ||
		strings.Contains(lower, "administratornotrequired") ||
		strings.Contains(lower, "noadministrator") ||
		strings.Contains(lower, "donotcontactadministrator") {
		return false
	}
	return strings.Contains(normalized, "需要联系管理员") ||
		strings.Contains(normalized, "联系管理员处理") ||
		strings.Contains(lower, "contactadministrator") ||
		strings.Contains(lower, "administratorhelpisrequired")
}

func normalizeTroubleshootingLocale(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	for _, part := range strings.Split(raw, ",") {
		tag := strings.TrimSpace(part)
		if semi := strings.IndexByte(tag, ';'); semi >= 0 {
			tag = strings.TrimSpace(tag[:semi])
		}
		switch {
		case tag == "en" || strings.HasPrefix(tag, "en-") || strings.HasPrefix(tag, "en_"):
			return troubleshootingLocaleEnglish
		case tag == "zh" || strings.HasPrefix(tag, "zh-") || strings.HasPrefix(tag, "zh_"):
			return troubleshootingLocaleChinese
		}
	}
	return troubleshootingLocaleChinese
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func truncateRunes(value string, max int) string {
	if max <= 0 || utf8.RuneCountInString(value) <= max {
		return value
	}
	runes := []rune(value)
	return string(runes[:max])
}

type RedisTroubleshootingRateLimiter struct {
	rdb *redis.Client
}

func ProvideTroubleshootingRateLimiter(rdb *redis.Client) *RedisTroubleshootingRateLimiter {
	return &RedisTroubleshootingRateLimiter{rdb: rdb}
}

func (l *RedisTroubleshootingRateLimiter) Allow(ctx context.Context, userID int64) (*TroubleshootingLimitState, error) {
	if l == nil || l.rdb == nil {
		return &TroubleshootingLimitState{
			ShortWindowRemaining: troubleshootingShortWindowLimit - 1,
			DailyRemaining:       troubleshootingDailyLimit - 1,
		}, nil
	}

	now := time.Now().UTC()
	shortKey := fmt.Sprintf("troubleshooting:rate:user:%d:5m:%d", userID, now.Unix()/int64(troubleshootingShortWindowDuration/time.Second))
	dayKey := fmt.Sprintf("troubleshooting:rate:user:%d:day:%s", userID, now.Format("20060102"))

	pipe := l.rdb.TxPipeline()
	shortIncr := pipe.Incr(ctx, shortKey)
	pipe.Expire(ctx, shortKey, troubleshootingShortWindowDuration+time.Minute)
	dailyIncr := pipe.Incr(ctx, dayKey)
	pipe.Expire(ctx, dayKey, troubleshootingDailyDuration+time.Hour)
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, err
	}

	shortCount := int(shortIncr.Val())
	dailyCount := int(dailyIncr.Val())
	state := &TroubleshootingLimitState{
		ShortWindowRemaining: maxTroubleshootingInt(0, troubleshootingShortWindowLimit-shortCount),
		DailyRemaining:       maxTroubleshootingInt(0, troubleshootingDailyLimit-dailyCount),
	}
	if shortCount > troubleshootingShortWindowLimit {
		return nil, infraerrors.TooManyRequests("TROUBLESHOOTING_RATE_LIMITED", "故障排查请求过于频繁，请稍后再试")
	}
	if dailyCount > troubleshootingDailyLimit {
		return nil, infraerrors.TooManyRequests("TROUBLESHOOTING_DAILY_LIMITED", "今日故障排查次数已达到限制，如仍无法解决请联系管理员")
	}
	return state, nil
}

func maxTroubleshootingInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

type OpenAITroubleshootingAIClient struct {
	accountRepo AccountRepository
	gateway     *OpenAIGatewayService
}

func NewOpenAITroubleshootingAIClient(accountRepo AccountRepository, gateway *OpenAIGatewayService) *OpenAITroubleshootingAIClient {
	return &OpenAITroubleshootingAIClient{
		accountRepo: accountRepo,
		gateway:     gateway,
	}
}

func (c *OpenAITroubleshootingAIClient) Diagnose(ctx context.Context, report string, localHint string, locale string) (string, int, error) {
	if c == nil || c.accountRepo == nil || c.gateway == nil || c.gateway.httpUpstream == nil {
		return "", 0, errors.New("troubleshooting ai client is not configured")
	}
	accounts, err := c.accountRepo.ListActive(ctx)
	if err != nil {
		return "", 0, err
	}

	candidates := troubleshootingAICandidates(accounts)
	if len(candidates) > troubleshootingMaxAIAccounts {
		candidates = candidates[:troubleshootingMaxAIAccounts]
	}
	if len(candidates) == 0 {
		return "", 0, errors.New("no available troubleshooting ai accounts")
	}

	prompt := buildTroubleshootingAIPrompt(report, localHint, locale)
	var lastErr error
	for i := range candidates {
		account := &candidates[i]
		answer, err := c.callAccount(ctx, account, prompt)
		if err == nil && strings.TrimSpace(answer) != "" {
			return answer, i + 1, nil
		}
		if err != nil {
			lastErr = err
		}
	}
	if lastErr == nil {
		lastErr = errors.New("empty troubleshooting ai response")
	}
	return "", len(candidates), lastErr
}

func troubleshootingAICandidates(accounts []Account) []Account {
	candidates := make([]Account, 0, len(accounts))
	for _, account := range accounts {
		if account.Platform != PlatformOpenAI {
			continue
		}
		if account.Type != AccountTypeOAuth && account.Type != AccountTypeAPIKey {
			continue
		}
		if !account.IsSchedulable() {
			continue
		}
		if account.Type == AccountTypeOAuth && strings.TrimSpace(account.GetOpenAIAccessToken()) == "" && strings.TrimSpace(account.GetOpenAIRefreshToken()) == "" {
			continue
		}
		if account.Type == AccountTypeAPIKey && strings.TrimSpace(account.GetOpenAIApiKey()) == "" {
			continue
		}
		candidates = append(candidates, account)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Priority == candidates[j].Priority {
			return candidates[i].ID < candidates[j].ID
		}
		return candidates[i].Priority < candidates[j].Priority
	})
	return candidates
}

func buildTroubleshootingAIPrompt(report string, localHint string, locale string) string {
	if normalizeTroubleshootingLocale(locale) == troubleshootingLocaleEnglish {
		return strings.Join([]string{
			"You are the built-in 51Token troubleshooting assistant. You can only diagnose API/client request failures.",
			"Do not answer writing, coding, translation, small talk, general knowledge, bypassing restrictions, or any non-troubleshooting request.",
			"Respond in English. The answer must include \"Diagnosis Result\" and \"User Action\". Add \"Contact Administrator\" and \"Information for Administrator\" only when administrator help is actually required. Do not say that an administrator is not needed.",
			"Do not list multiple possible causes. Give only the single most likely conclusion.",
			"If local rules or system evidence already provide a clear conclusion, restate that conclusion without expanding into guesses.",
			"If there is no system evidence, explicitly say \"The current system did not find an exact failure record\" and ask the user to retry or provide a request id.",
			"Do not invent backend state, account-pool state, Cloudflare state, or upstream logs.",
			"",
			"System evidence / local rules:",
			localHint,
			"",
			"User pasted error:",
			report,
		}, "\n")
	}
	return strings.Join([]string{
		"你是 51Token 系统内置的故障排查助手，只能排查 API/客户端请求失败原因。",
		"禁止回答写作、代码、翻译、闲聊、通用知识、绕过限制等非故障排查问题。",
		"请用中文输出，必须包含“排查结果”和“用户可执行操作”。只有确认需要管理员处理时，才增加“需要联系管理员”和“建议提供给管理员的信息”；不需要时不要输出“是否需要联系管理员”或“暂时不需要联系管理员”。",
		"不要罗列多个可能原因；只能给出一个最可能结论。",
		"如果本地规则或系统证据已经给出明确结论，直接复述该结论，不要扩展猜测。",
		"如果没有系统证据，必须明确写“当前系统未查到精确失败记录”，并要求用户重试或提供 request id。",
		"不要编造后台状态、账号池状态、Cloudflare 状态或上游日志。",
		"",
		"系统证据/本地规则：",
		localHint,
		"",
		"用户粘贴的错误：",
		report,
	}, "\n")
}

func (c *OpenAITroubleshootingAIClient) callAccount(ctx context.Context, account *Account, prompt string) (string, error) {
	token, _, err := c.gateway.GetAccessToken(ctx, account)
	if err != nil {
		return "", err
	}
	body, err := buildTroubleshootingAIRequestBody(account, prompt)
	if err != nil {
		return "", err
	}

	req, err := c.gateway.buildUpstreamRequest(ctx, nil, account, body, token, false, "", false)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Content-Type", "application/json")
	proxyURL := c.gateway.openAICodexHTTPProxyURL(account, req)
	resp, err := c.gateway.httpUpstream.Do(req, proxyURL, account.ID, account.Concurrency)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 400 {
		msg := strings.TrimSpace(extractUpstreamErrorMessage(respBody))
		if msg == "" {
			msg = http.StatusText(resp.StatusCode)
		}
		return "", fmt.Errorf("troubleshooting ai upstream returned %d: %s", resp.StatusCode, msg)
	}
	if answer := extractTroubleshootingAIText(respBody); answer != "" {
		return answer, nil
	}
	return "", fmt.Errorf("troubleshooting ai response has no text: %s", truncateRunes(string(bytes.TrimSpace(respBody)), 300))
}

func buildTroubleshootingAIRequestBody(account *Account, prompt string) ([]byte, error) {
	payload := map[string]any{
		"model": troubleshootingAIModelForAccount(account),
		"input": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{
						"type": "input_text",
						"text": prompt,
					},
				},
			},
		},
		"store":            false,
		"stream":           true,
		"prompt_cache_key": fmt.Sprintf("troubleshooting-%d", account.ID),
	}
	if account == nil || account.Type != AccountTypeOAuth {
		payload["max_output_tokens"] = 700
	}
	return json.Marshal(payload)
}

func extractTroubleshootingAIText(body []byte) string {
	if text := strings.TrimSpace(gjson.GetBytes(body, "output_text").String()); text != "" {
		return text
	}
	if text := strings.TrimSpace(gjson.GetBytes(body, "response.output_text").String()); text != "" {
		return text
	}
	if text := extractTroubleshootingOutputText(gjson.GetBytes(body, "output")); text != "" {
		return text
	}
	if text := extractTroubleshootingOutputText(gjson.GetBytes(body, "response.output")); text != "" {
		return text
	}
	return extractTroubleshootingSSEText(body)
}

func extractTroubleshootingOutputText(output gjson.Result) string {
	if !output.Exists() {
		return ""
	}
	var parts []string
	output.ForEach(func(_, item gjson.Result) bool {
		content := item.Get("content")
		content.ForEach(func(_, contentItem gjson.Result) bool {
			if text := strings.TrimSpace(contentItem.Get("text").String()); text != "" {
				parts = append(parts, text)
			}
			return true
		})
		return true
	})
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func extractTroubleshootingSSEText(body []byte) string {
	var deltas []string
	var final string
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		raw := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if raw == "" || raw == "[DONE]" {
			continue
		}
		event := gjson.Parse(raw)
		if delta := event.Get("delta"); delta.Exists() {
			deltas = append(deltas, delta.String())
			continue
		}
		if text := extractTroubleshootingOutputText(event.Get("response.output")); text != "" {
			final = text
		}
	}
	if len(deltas) > 0 {
		return strings.TrimSpace(strings.Join(deltas, ""))
	}
	return strings.TrimSpace(final)
}

func troubleshootingAIModelForAccount(account *Account) string {
	if account != nil {
		for _, model := range []string{"gpt-5.3-codex-spark", "gpt-5.3-codex", "gpt-5.4", "gpt-5.5"} {
			if mapped, matched := account.ResolveMappedModel(model); matched && strings.TrimSpace(mapped) != "" {
				return strings.TrimSpace(mapped)
			}
		}
		if account.Type == AccountTypeOAuth {
			return "gpt-5.3-codex-spark"
		}
	}
	return "gpt-5.3-codex-spark"
}
