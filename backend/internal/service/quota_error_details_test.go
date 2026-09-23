package service

import (
	"context"
	"errors"
	"testing"
	"time"

	pkgerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestQuotaErrorDetails(t *testing.T) {
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	reset := now.Add(3*24*time.Hour + 2*time.Hour + 15*time.Minute)
	for _, tc := range []struct {
		err   error
		label string
	}{
		{ErrDailyLimitExceeded, "日"}, {ErrWeeklyLimitExceeded, "周"}, {ErrMonthlyLimitExceeded, "月"},
		{ErrUserPlatformDailyQuotaExhausted, "日"}, {ErrUserPlatformWeeklyQuotaExhausted, "周"}, {ErrUserPlatformMonthlyQuotaExhausted, "月"},
		{ErrAPIKeyRateLimit5hExceeded, "5小时"}, {ErrAPIKeyRateLimit1dExceeded, "日"}, {ErrAPIKeyRateLimit7dExceeded, "7天"},
	} {
		t.Run(pkgerrors.Reason(tc.err), func(t *testing.T) {
			err := quotaErrorDetails(tc.err, 200, 201, &reset, reset.Add(time.Hour), now)
			require.True(t, errors.Is(err, tc.err))
			require.Equal(t, reset.Format(time.RFC3339), pkgerrors.FromError(err).Metadata["window_resets_at"])
			message := ClientErrorMessageForAcceptLanguage("zh", pkgerrors.Message(err))
			require.Contains(t, message, tc.label+"限额 $200")
			require.Contains(t, message, "3 天 2 小时 15 分钟")
			require.NotContains(t, message, "上游")
			require.Contains(t, ClientErrorMessageForAcceptLanguage("en", pkgerrors.Message(err)), "3 days 2 hours 15 minutes")
			require.Empty(t, pkgerrors.FromError(tc.err).Metadata)
		})
	}
	for _, tc := range []struct {
		name    string
		err     error
		reset   *time.Time
		expires time.Time
		want    string
	}{
		{"total", ErrTotalLimitExceeded, nil, time.Time{}, "不会自动恢复"},
		{"expiry", ErrWeeklyLimitExceeded, &reset, reset, "重置前到期"},
		{"unknown", ErrWeeklyLimitExceeded, nil, time.Time{}, "暂无法确定"},
		{"past", ErrWeeklyLimitExceeded, &now, time.Time{}, "暂无法确定"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := quotaErrorDetails(tc.err, 200, 200, tc.reset, tc.expires, now)
			require.Contains(t, pkgerrors.Message(err), tc.want)
			require.Empty(t, pkgerrors.FromError(err).Metadata["window_resets_at"])
		})
	}
}

type quotaDetailsCache struct {
	billingCacheWorkerStub
	data SubscriptionCacheData
}

func (c *quotaDetailsCache) GetSubscriptionCache(context.Context, int64, int64) (*SubscriptionCacheData, error) {
	return &c.data, nil
}

func TestSubscriptionQuotaDetailsFromBillingCache(t *testing.T) {
	limit := 200.0
	start := time.Now().Add(-24 * time.Hour)
	sub := &UserSubscription{WeeklyWindowStart: &start, ExpiresAt: time.Now().Add(30 * 24 * time.Hour)}
	cache := &quotaDetailsCache{data: SubscriptionCacheData{Status: SubscriptionStatusActive, ExpiresAt: sub.ExpiresAt, WeeklyUsage: 201}}
	svc := &BillingCacheService{cache: cache}
	err := svc.checkSubscriptionEligibility(context.Background(), 1, &Group{WeeklyLimitUSD: &limit}, sub)
	require.ErrorIs(t, err, ErrWeeklyLimitExceeded)
	require.Contains(t, pkgerrors.Message(err), "周限额 $200")
	require.Equal(t, "201", pkgerrors.FromError(err).Metadata["used_usd"])
	require.NotEmpty(t, pkgerrors.FromError(err).Metadata["window_resets_at"])
	cache.data.ExpiresAt = time.Now().Add(-time.Hour)
	require.ErrorIs(t, svc.checkSubscriptionEligibility(context.Background(), 1, &Group{}, sub), ErrSubscriptionExpired)
	cache.data.ExpiresAt = sub.ExpiresAt
	cache.data.Status = SubscriptionStatusSuspended
	require.ErrorIs(t, svc.checkSubscriptionEligibility(context.Background(), 1, &Group{}, sub), ErrSubscriptionSuspended)
}
