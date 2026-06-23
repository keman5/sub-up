//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/stretchr/testify/require"
)

func TestResolvePresentationMultiplier(t *testing.T) {
	t.Parallel()

	t.Run("disabled group uses one", func(t *testing.T) {
		got := ResolvePresentationMultiplier(&Group{
			UsageMultiplierEnabled: false,
			UsageMultiplier:        2,
		}, 700, 300, 0, 0)
		require.Equal(t, 1.0, got)
	})

	t.Run("enabled below threshold uses one", func(t *testing.T) {
		got := ResolvePresentationMultiplier(&Group{
			UsageMultiplierEnabled: true,
			UsageMultiplier:        2,
		}, 500, 499, 0, 0)
		require.Equal(t, 1.0, got)
	})

	t.Run("enabled at threshold uses group multiplier", func(t *testing.T) {
		got := ResolvePresentationMultiplier(&Group{
			UsageMultiplierEnabled: true,
			UsageMultiplier:        2,
		}, 600, 300, 50, 50)
		require.Equal(t, 2.0, got)
	})

	t.Run("image output only reaches threshold", func(t *testing.T) {
		got := ResolvePresentationMultiplierWithImageOutput(&Group{
			UsageMultiplierEnabled: true,
			UsageMultiplier:        2,
		}, 0, 0, 0, 0, 1000)
		require.Equal(t, 2.0, got)
	})

	t.Run("image output included in output is not double counted", func(t *testing.T) {
		got := ResolvePresentationMultiplierWithImageOutput(&Group{
			UsageMultiplierEnabled: true,
			UsageMultiplier:        2,
		}, 0, 600, 0, 0, 600)
		require.Equal(t, 1.0, got)
	})
}

func TestUsageViewModeForRole(t *testing.T) {
	t.Parallel()

	require.Equal(t, UsageViewRaw, UsageViewModeForRole(RoleSuperAdmin))
	require.Equal(t, UsageViewPresentation, UsageViewModeForRole(RoleAdmin))
	require.Equal(t, UsageViewPresentation, UsageViewModeForRole(RoleUser))
	require.Equal(t, UsageViewPresentation, UsageViewModeForRole(""))
}

func TestUsageLogForView(t *testing.T) {
	t.Parallel()

	log := &UsageLog{
		InputTokens:            500,
		OutputTokens:           600,
		CacheCreationTokens:    100,
		CacheReadTokens:        50,
		TotalCost:              0.05,
		ActualCost:             0.1,
		RateMultiplier:         1.7,
		PresentationMultiplier: 2,
	}

	presentation := UsageLogForView(log, UsageViewPresentation)
	require.Equal(t, 1000, presentation.InputTokens)
	require.Equal(t, 1200, presentation.OutputTokens)
	require.Equal(t, 200, presentation.CacheCreationTokens)
	require.Equal(t, 100, presentation.CacheReadTokens)
	require.InDelta(t, 0.1, presentation.TotalCost, 1e-12)
	require.InDelta(t, 0.2, presentation.ActualCost, 1e-12)
	require.InDelta(t, 1.0, presentation.RateMultiplier, 1e-12)

	raw := UsageLogForView(log, UsageViewRaw)
	require.Equal(t, 500, raw.InputTokens)
	require.Equal(t, 600, raw.OutputTokens)
	require.InDelta(t, 0.05, raw.TotalCost, 1e-12)
	require.InDelta(t, 0.1, raw.ActualCost, 1e-12)
	require.InDelta(t, 1.7, raw.RateMultiplier, 1e-12)

	require.Equal(t, 500, log.InputTokens, "must not mutate stored log")
	require.InDelta(t, 1.7, log.RateMultiplier, 1e-12)
}

type batchAPIKeyUsageStatsViewRepoStub struct {
	UsageLogRepository
	usePresentation bool
}

func (s *batchAPIKeyUsageStatsViewRepoStub) GetBatchAPIKeyUsageStatsForView(ctx context.Context, apiKeyIDs []int64, startTime, endTime time.Time, usePresentation bool) (map[int64]*usagestats.BatchAPIKeyUsageStats, error) {
	s.usePresentation = usePresentation
	return map[int64]*usagestats.BatchAPIKeyUsageStats{
		7: {APIKeyID: 7, TotalActualCost: 0.2, TodayActualCost: 0.1},
	}, nil
}

func TestUsageServiceGetBatchAPIKeyUsageStatsUsesPresentationView(t *testing.T) {
	t.Parallel()

	repo := &batchAPIKeyUsageStatsViewRepoStub{}
	svc := NewUsageService(repo, nil, nil, nil)

	got, err := svc.GetBatchAPIKeyUsageStats(context.Background(), []int64{7}, time.Time{}, time.Time{})

	require.NoError(t, err)
	require.True(t, repo.usePresentation)
	require.InDelta(t, 0.2, got[7].TotalActualCost, 1e-12)
}

func TestUsageLogForView_DefaultsInvalidPresentationMultiplierToOne(t *testing.T) {
	t.Parallel()

	log := &UsageLog{
		InputTokens:            500,
		TotalCost:              0.05,
		RateMultiplier:         1.7,
		PresentationMultiplier: 0,
	}

	got := UsageLogForView(log, UsageViewPresentation)
	require.Equal(t, 500, got.InputTokens)
	require.InDelta(t, 0.05, got.TotalCost, 1e-12)
	require.InDelta(t, 1.0, got.RateMultiplier, 1e-12)
}
