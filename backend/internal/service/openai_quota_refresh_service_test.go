package service

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type quotaRefreshLifecycleRepo struct {
	AccountRepository
	calls    atomic.Int64
	accounts []Account
}

func (r *quotaRefreshLifecycleRepo) ListByPlatform(_ context.Context, platform string) ([]Account, error) {
	if platform != PlatformOpenAI {
		panic("unexpected quota refresh platform")
	}
	r.calls.Add(1)
	return r.accounts, nil
}

func TestProvideOpenAIQuotaRefreshService_StartsWithoutTraffic(t *testing.T) {
	repo := &quotaRefreshLifecycleRepo{}
	svc := ProvideOpenAIQuotaRefreshService(repo, &AccountUsageService{})
	t.Cleanup(svc.Stop)
	require.Equal(t, openAIQuotaRefreshInterval, svc.interval)
	require.Eventually(t, func() bool { return repo.calls.Load() > 0 }, time.Second, time.Millisecond)
	svc.Stop()
	svc.Stop() // cleanup remains safe after an explicit stop.
}

func TestOpenAIQuotaRefreshService_StartPeriodicRefreshAndStop(t *testing.T) {
	repo := &quotaRefreshLifecycleRepo{accounts: []Account{{
		ID: 42, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive,
		Extra: map[string]any{"openai_oauth_responses_websockets_v2_enabled": true},
	}}}
	svc := NewOpenAIQuotaRefreshService(repo, &AccountUsageService{}, 5*time.Millisecond)
	var refreshed atomic.Int64
	svc.refreshAccountUsage = func(_ context.Context, id int64) error {
		if id != 42 {
			panic("unexpected refresh account")
		}
		refreshed.Add(1)
		return nil
	}
	svc.Start()
	t.Cleanup(svc.Stop)
	require.Eventually(t, func() bool { return refreshed.Load() >= 2 }, time.Second, time.Millisecond)
	svc.Stop()
	count := refreshed.Load()
	require.Never(t, func() bool { return refreshed.Load() != count }, 30*time.Millisecond, time.Millisecond)
}
