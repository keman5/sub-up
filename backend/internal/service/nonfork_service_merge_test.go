//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	pkgerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestNonforkPendingRateLimitKeepsQuotaDetails(t *testing.T) {
	start := time.Now()
	svc := &BillingCacheService{}
	for _, tc := range []struct {
		name string
		key  APIKey
		err  error
	}{
		{"5h", APIKey{RateLimit5h: 10}, ErrAPIKeyRateLimit5hExceeded},
		{"1d", APIKey{RateLimit1d: 10}, ErrAPIKeyRateLimit1dExceeded},
		{"7d", APIKey{RateLimit7d: 10}, ErrAPIKeyRateLimit7dExceeded},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, svc.evaluateRateLimits(context.Background(), &tc.key, 9, 9, 9, &start, &start, &start, 1))
			err := svc.evaluateRateLimits(context.Background(), &tc.key, 9, 9, 9, &start, &start, &start, 2)
			require.ErrorIs(t, err, tc.err)
			details := pkgerrors.FromError(err).Metadata
			require.Equal(t, "10", details["limit_usd"])
			require.Equal(t, "9", details["used_usd"])
			require.NotEmpty(t, details["window_resets_at"])
		})
	}
}

func TestNonforkUsageKeepsCacheTTLAndImageFallback(t *testing.T) {
	usage, ok := openAIUsageFromGJSON(gjson.Parse(`{"input_tokens":100,"output_tokens":20,"image_output_tokens":7,"input_tokens_details":{"cache_write_tokens":10,"cache_write_tokens_details":{"ttl_30m_tokens":3,"ttl_12h_tokens":7}}}`))
	require.True(t, ok)
	require.Equal(t, 10, usage.CacheCreationInputTokens)
	require.Equal(t, 3, usage.CacheCreation30mInputTokens)
	require.Equal(t, 7, usage.CacheCreation12hInputTokens)
	require.Equal(t, 7, usage.ImageOutputTokens)
}

func TestNonforkPresentationBatchKeepsScopedAccessCheck(t *testing.T) {
	repo := &scopedStatisticsBatchRepo{stubQuotaAccountRepo: stubQuotaAccountRepo{accounts: map[int64]*Account{
		1: {ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth, GroupIDs: []int64{1}},
		2: {ID: 2, Platform: PlatformOpenAI, Type: AccountTypeOAuth, GroupIDs: []int64{2}},
	}}}
	logs := &scopedStatisticsLogRepo{}
	svc := &AccountUsageService{accountRepo: repo, usageLogRepo: logs}
	_, err := svc.GetTodayStatsBatchForView(scopedStatisticsServiceContext("accounts.stats.read"), []int64{1, 2}, UsageViewPresentation)
	require.Equal(t, 404, pkgerrors.Code(err))
	require.Zero(t, logs.reads)
}

func TestNonforkAgentIdentityProbeDoesNotRequireOAuthToken(t *testing.T) {
	account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth,
		Credentials: map[string]any{"auth_mode": OpenAIAuthModeAgentIdentity}}
	_, err := (&AccountUsageService{}).probeOpenAICodexSnapshotForModel(context.Background(), account, openAICodexMainProbeModel)
	require.Error(t, err)
	require.Contains(t, err.Error(), "build Agent Identity authentication")
	require.NotContains(t, err.Error(), "no access token available")
}
