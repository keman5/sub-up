package service

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/stretchr/testify/require"
)

type usageRepoStub struct {
	UsageLogRepository
	stats            *usagestats.DashboardStats
	rangeStats       *usagestats.DashboardStats
	modelStats       []usagestats.ModelStat
	groupStats       []usagestats.GroupStat
	groupSummary     []usagestats.GroupUsageSummary
	userBreakdown    []usagestats.UserBreakdownItem
	err              error
	rangeErr         error
	calls            int32
	rangeCalls       int32
	rangeStart       time.Time
	rangeEnd         time.Time
	onCall           chan struct{}
	lastPresentation bool
}

func (s *usageRepoStub) GetDashboardStats(ctx context.Context) (*usagestats.DashboardStats, error) {
	atomic.AddInt32(&s.calls, 1)
	if s.onCall != nil {
		select {
		case s.onCall <- struct{}{}:
		default:
		}
	}
	if s.err != nil {
		return nil, s.err
	}
	return s.stats, nil
}

func (s *usageRepoStub) GetDashboardStatsWithRange(ctx context.Context, start, end time.Time) (*usagestats.DashboardStats, error) {
	atomic.AddInt32(&s.rangeCalls, 1)
	s.rangeStart = start
	s.rangeEnd = end
	if s.rangeErr != nil {
		return nil, s.rangeErr
	}
	if s.rangeStats != nil {
		return s.rangeStats, nil
	}
	return s.stats, nil
}

func (s *usageRepoStub) GetDashboardStatsForView(ctx context.Context, usePresentation bool) (*usagestats.DashboardStats, error) {
	s.lastPresentation = usePresentation
	return s.GetDashboardStats(ctx)
}

func (s *usageRepoStub) GetModelStatsWithFiltersBySourceForView(ctx context.Context, startTime, endTime time.Time, userID, apiKeyID, accountID, groupID int64, requestType *int16, stream *bool, billingType *int8, source string, usePresentation bool) ([]usagestats.ModelStat, error) {
	s.lastPresentation = usePresentation
	return s.modelStats, nil
}

func (s *usageRepoStub) GetGroupStatsWithFiltersForView(ctx context.Context, startTime, endTime time.Time, userID, apiKeyID, accountID, groupID int64, requestType *int16, stream *bool, billingType *int8, usePresentation bool) ([]usagestats.GroupStat, error) {
	s.lastPresentation = usePresentation
	return s.groupStats, nil
}

func (s *usageRepoStub) GetAllGroupUsageSummaryForView(ctx context.Context, todayStart time.Time, usePresentation bool) ([]usagestats.GroupUsageSummary, error) {
	s.lastPresentation = usePresentation
	return s.groupSummary, nil
}

func (s *usageRepoStub) GetUserBreakdownStatsForView(ctx context.Context, startTime, endTime time.Time, dim usagestats.UserBreakdownDimension, limit int, usePresentation bool) ([]usagestats.UserBreakdownItem, error) {
	s.lastPresentation = usePresentation
	return s.userBreakdown, nil
}

type dashboardCacheStub struct {
	get       func(ctx context.Context) (string, error)
	set       func(ctx context.Context, data string, ttl time.Duration) error
	del       func(ctx context.Context) error
	getCalls  int32
	setCalls  int32
	delCalls  int32
	lastSetMu sync.Mutex
	lastSet   string
}

func (c *dashboardCacheStub) GetDashboardStats(ctx context.Context) (string, error) {
	atomic.AddInt32(&c.getCalls, 1)
	if c.get != nil {
		return c.get(ctx)
	}
	return "", ErrDashboardStatsCacheMiss
}

func (c *dashboardCacheStub) SetDashboardStats(ctx context.Context, data string, ttl time.Duration) error {
	atomic.AddInt32(&c.setCalls, 1)
	c.lastSetMu.Lock()
	c.lastSet = data
	c.lastSetMu.Unlock()
	if c.set != nil {
		return c.set(ctx, data, ttl)
	}
	return nil
}

func (c *dashboardCacheStub) DeleteDashboardStats(ctx context.Context) error {
	atomic.AddInt32(&c.delCalls, 1)
	if c.del != nil {
		return c.del(ctx)
	}
	return nil
}

type dashboardAggregationRepoStub struct {
	watermark time.Time
	err       error
}

func (s *dashboardAggregationRepoStub) AggregateRange(ctx context.Context, start, end time.Time) error {
	return nil
}

func (s *dashboardAggregationRepoStub) RecomputeRange(ctx context.Context, start, end time.Time) error {
	return nil
}

func (s *dashboardAggregationRepoStub) GetAggregationWatermark(ctx context.Context) (time.Time, error) {
	if s.err != nil {
		return time.Time{}, s.err
	}
	return s.watermark, nil
}

func (s *dashboardAggregationRepoStub) UpdateAggregationWatermark(ctx context.Context, aggregatedAt time.Time) error {
	return nil
}

func (s *dashboardAggregationRepoStub) CleanupAggregates(ctx context.Context, hourlyCutoff, dailyCutoff time.Time) error {
	return nil
}

func (s *dashboardAggregationRepoStub) CleanupUsageLogs(ctx context.Context, cutoff time.Time) error {
	return nil
}

func (s *dashboardAggregationRepoStub) CleanupUsageBillingDedup(ctx context.Context, cutoff time.Time) error {
	return nil
}

func (s *dashboardAggregationRepoStub) EnsureUsageLogsPartitions(ctx context.Context, now time.Time) error {
	return nil
}

func (c *dashboardCacheStub) readLastEntry(t *testing.T) dashboardStatsCacheEntry {
	t.Helper()
	c.lastSetMu.Lock()
	data := c.lastSet
	c.lastSetMu.Unlock()

	var entry dashboardStatsCacheEntry
	err := json.Unmarshal([]byte(data), &entry)
	require.NoError(t, err)
	return entry
}

func TestDashboardService_CacheHitFresh(t *testing.T) {
	stats := &usagestats.DashboardStats{
		TotalUsers:     10,
		StatsUpdatedAt: time.Unix(0, 0).UTC().Format(time.RFC3339),
		StatsStale:     true,
	}
	entry := dashboardStatsCacheEntry{
		Stats:     stats,
		UpdatedAt: time.Now().Unix(),
	}
	payload, err := json.Marshal(entry)
	require.NoError(t, err)

	cache := &dashboardCacheStub{
		get: func(ctx context.Context) (string, error) {
			return string(payload), nil
		},
	}
	repo := &usageRepoStub{
		stats: &usagestats.DashboardStats{TotalUsers: 99},
	}
	aggRepo := &dashboardAggregationRepoStub{watermark: time.Unix(0, 0).UTC()}
	cfg := &config.Config{
		Dashboard: config.DashboardCacheConfig{Enabled: true},
		DashboardAgg: config.DashboardAggregationConfig{
			Enabled: true,
		},
	}
	svc := NewDashboardService(repo, aggRepo, cache, cfg)

	got, err := svc.GetDashboardStats(context.Background())
	require.NoError(t, err)
	require.Equal(t, stats, got)
	require.Equal(t, int32(0), atomic.LoadInt32(&repo.calls))
	require.Equal(t, int32(1), atomic.LoadInt32(&cache.getCalls))
	require.Equal(t, int32(0), atomic.LoadInt32(&cache.setCalls))
}

func TestDashboardService_CacheMiss_StoresCache(t *testing.T) {
	stats := &usagestats.DashboardStats{
		TotalUsers:     7,
		StatsUpdatedAt: time.Unix(0, 0).UTC().Format(time.RFC3339),
		StatsStale:     true,
	}
	cache := &dashboardCacheStub{
		get: func(ctx context.Context) (string, error) {
			return "", ErrDashboardStatsCacheMiss
		},
	}
	repo := &usageRepoStub{stats: stats}
	aggRepo := &dashboardAggregationRepoStub{watermark: time.Unix(0, 0).UTC()}
	cfg := &config.Config{
		Dashboard: config.DashboardCacheConfig{Enabled: true},
		DashboardAgg: config.DashboardAggregationConfig{
			Enabled: true,
		},
	}
	svc := NewDashboardService(repo, aggRepo, cache, cfg)

	got, err := svc.GetDashboardStats(context.Background())
	require.NoError(t, err)
	require.Equal(t, stats, got)
	require.Equal(t, int32(1), atomic.LoadInt32(&repo.calls))
	require.Equal(t, int32(1), atomic.LoadInt32(&cache.getCalls))
	require.Equal(t, int32(1), atomic.LoadInt32(&cache.setCalls))
	entry := cache.readLastEntry(t)
	require.Equal(t, stats, entry.Stats)
	require.WithinDuration(t, time.Now(), time.Unix(entry.UpdatedAt, 0), time.Second)
}

func TestDashboardService_CacheDisabled_SkipsCache(t *testing.T) {
	stats := &usagestats.DashboardStats{
		TotalUsers:     3,
		StatsUpdatedAt: time.Unix(0, 0).UTC().Format(time.RFC3339),
		StatsStale:     true,
	}
	cache := &dashboardCacheStub{
		get: func(ctx context.Context) (string, error) {
			return "", nil
		},
	}
	repo := &usageRepoStub{stats: stats}
	aggRepo := &dashboardAggregationRepoStub{watermark: time.Unix(0, 0).UTC()}
	cfg := &config.Config{
		Dashboard: config.DashboardCacheConfig{Enabled: false},
		DashboardAgg: config.DashboardAggregationConfig{
			Enabled: true,
		},
	}
	svc := NewDashboardService(repo, aggRepo, cache, cfg)

	got, err := svc.GetDashboardStats(context.Background())
	require.NoError(t, err)
	require.Equal(t, stats, got)
	require.Equal(t, int32(1), atomic.LoadInt32(&repo.calls))
	require.Equal(t, int32(0), atomic.LoadInt32(&cache.getCalls))
	require.Equal(t, int32(0), atomic.LoadInt32(&cache.setCalls))
}

func TestDashboardService_CacheHitStale_TriggersAsyncRefresh(t *testing.T) {
	staleStats := &usagestats.DashboardStats{
		TotalUsers:     11,
		StatsUpdatedAt: time.Unix(0, 0).UTC().Format(time.RFC3339),
		StatsStale:     true,
	}
	entry := dashboardStatsCacheEntry{
		Stats:     staleStats,
		UpdatedAt: time.Now().Add(-defaultDashboardStatsFreshTTL * 2).Unix(),
	}
	payload, err := json.Marshal(entry)
	require.NoError(t, err)

	cache := &dashboardCacheStub{
		get: func(ctx context.Context) (string, error) {
			return string(payload), nil
		},
	}
	refreshCh := make(chan struct{}, 1)
	repo := &usageRepoStub{
		stats:  &usagestats.DashboardStats{TotalUsers: 22},
		onCall: refreshCh,
	}
	aggRepo := &dashboardAggregationRepoStub{watermark: time.Unix(0, 0).UTC()}
	cfg := &config.Config{
		Dashboard: config.DashboardCacheConfig{Enabled: true},
		DashboardAgg: config.DashboardAggregationConfig{
			Enabled: true,
		},
	}
	svc := NewDashboardService(repo, aggRepo, cache, cfg)

	got, err := svc.GetDashboardStats(context.Background())
	require.NoError(t, err)
	require.Equal(t, staleStats, got)

	select {
	case <-refreshCh:
	case <-time.After(1 * time.Second):
		t.Fatal("等待异步刷新超时")
	}
	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&cache.setCalls) >= 1
	}, 1*time.Second, 10*time.Millisecond)
}

func TestDashboardService_CacheParseError_EvictsAndRefetches(t *testing.T) {
	cache := &dashboardCacheStub{
		get: func(ctx context.Context) (string, error) {
			return "not-json", nil
		},
	}
	stats := &usagestats.DashboardStats{TotalUsers: 9}
	repo := &usageRepoStub{stats: stats}
	aggRepo := &dashboardAggregationRepoStub{watermark: time.Unix(0, 0).UTC()}
	cfg := &config.Config{
		Dashboard: config.DashboardCacheConfig{Enabled: true},
		DashboardAgg: config.DashboardAggregationConfig{
			Enabled: true,
		},
	}
	svc := NewDashboardService(repo, aggRepo, cache, cfg)

	got, err := svc.GetDashboardStats(context.Background())
	require.NoError(t, err)
	require.Equal(t, stats, got)
	require.Equal(t, int32(1), atomic.LoadInt32(&cache.delCalls))
	require.Equal(t, int32(1), atomic.LoadInt32(&repo.calls))
}

func TestDashboardService_CacheParseError_RepoFailure(t *testing.T) {
	cache := &dashboardCacheStub{
		get: func(ctx context.Context) (string, error) {
			return "not-json", nil
		},
	}
	repo := &usageRepoStub{err: errors.New("db down")}
	aggRepo := &dashboardAggregationRepoStub{watermark: time.Unix(0, 0).UTC()}
	cfg := &config.Config{
		Dashboard: config.DashboardCacheConfig{Enabled: true},
		DashboardAgg: config.DashboardAggregationConfig{
			Enabled: true,
		},
	}
	svc := NewDashboardService(repo, aggRepo, cache, cfg)

	_, err := svc.GetDashboardStats(context.Background())
	require.Error(t, err)
	require.Equal(t, int32(1), atomic.LoadInt32(&cache.delCalls))
}

func TestDashboardService_StatsUpdatedAtEpochWhenMissing(t *testing.T) {
	stats := &usagestats.DashboardStats{}
	repo := &usageRepoStub{stats: stats}
	aggRepo := &dashboardAggregationRepoStub{watermark: time.Unix(0, 0).UTC()}
	cfg := &config.Config{Dashboard: config.DashboardCacheConfig{Enabled: false}}
	svc := NewDashboardService(repo, aggRepo, nil, cfg)

	got, err := svc.GetDashboardStats(context.Background())
	require.NoError(t, err)
	require.Equal(t, "1970-01-01T00:00:00Z", got.StatsUpdatedAt)
	require.True(t, got.StatsStale)
}

func TestDashboardService_StatsStaleFalseWhenFresh(t *testing.T) {
	aggNow := time.Now().UTC().Truncate(time.Second)
	stats := &usagestats.DashboardStats{}
	repo := &usageRepoStub{stats: stats}
	aggRepo := &dashboardAggregationRepoStub{watermark: aggNow}
	cfg := &config.Config{
		Dashboard: config.DashboardCacheConfig{Enabled: false},
		DashboardAgg: config.DashboardAggregationConfig{
			Enabled:         true,
			IntervalSeconds: 60,
			LookbackSeconds: 120,
		},
	}
	svc := NewDashboardService(repo, aggRepo, nil, cfg)

	got, err := svc.GetDashboardStats(context.Background())
	require.NoError(t, err)
	require.Equal(t, aggNow.Format(time.RFC3339), got.StatsUpdatedAt)
	require.False(t, got.StatsStale)
}

func TestDashboardService_AggDisabled_UsesUsageLogsFallback(t *testing.T) {
	expected := &usagestats.DashboardStats{TotalUsers: 42}
	repo := &usageRepoStub{
		rangeStats: expected,
		err:        errors.New("should not call aggregated stats"),
	}
	cfg := &config.Config{
		Dashboard: config.DashboardCacheConfig{Enabled: false},
		DashboardAgg: config.DashboardAggregationConfig{
			Enabled: false,
			Retention: config.DashboardAggregationRetentionConfig{
				UsageLogsDays: 7,
			},
		},
	}
	svc := NewDashboardService(repo, nil, nil, cfg)

	got, err := svc.GetDashboardStats(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(42), got.TotalUsers)
	require.Equal(t, int32(0), atomic.LoadInt32(&repo.calls))
	require.Equal(t, int32(1), atomic.LoadInt32(&repo.rangeCalls))
	require.False(t, repo.rangeEnd.IsZero())
	require.Equal(t, truncateToDayUTC(repo.rangeEnd.AddDate(0, 0, -7)), repo.rangeStart)
}

func TestDashboardService_GetDashboardStatsForViewMasksAccountCostForPresentation(t *testing.T) {
	stats := &usagestats.DashboardStats{
		TotalUsers:       42,
		TotalAccountCost: 12.5,
		TodayAccountCost: 3.25,
	}
	repo := &usageRepoStub{stats: stats}
	svc := NewDashboardService(repo, nil, nil, &config.Config{
		Dashboard: config.DashboardCacheConfig{Enabled: false},
	})

	got, err := svc.GetDashboardStatsForView(context.Background(), true)
	require.NoError(t, err)
	require.True(t, repo.lastPresentation)
	require.Equal(t, int64(42), got.TotalUsers)
	require.Zero(t, got.TotalAccountCost)
	require.Zero(t, got.TodayAccountCost)
	require.Equal(t, 12.5, stats.TotalAccountCost, "presentation view must not mutate raw stats")
	require.Equal(t, 3.25, stats.TodayAccountCost, "presentation view must not mutate raw stats")

	raw, err := svc.GetDashboardStatsForView(context.Background(), false)
	require.NoError(t, err)
	require.Equal(t, 12.5, raw.TotalAccountCost)
	require.Equal(t, 3.25, raw.TodayAccountCost)
}

func TestDashboardService_ForViewMasksAccountCostSlicesForPresentation(t *testing.T) {
	start := time.Now().Add(-time.Hour)
	end := time.Now()
	repo := &usageRepoStub{
		modelStats: []usagestats.ModelStat{{
			Model:       "gpt-5.5",
			AccountCost: 8.8,
		}},
		groupStats: []usagestats.GroupStat{{
			GroupID:     1,
			GroupName:   "standard",
			AccountCost: 9.9,
		}},
		userBreakdown: []usagestats.UserBreakdownItem{{
			UserID:      2,
			Email:       "u@example.com",
			AccountCost: 7.7,
		}},
	}
	svc := NewDashboardService(repo, nil, nil, &config.Config{})

	models, err := svc.GetModelStatsWithFiltersBySourceForView(context.Background(), start, end, 0, 0, 0, 0, nil, nil, nil, usagestats.ModelSourceRequested, true)
	require.NoError(t, err)
	require.Len(t, models, 1)
	require.Zero(t, models[0].AccountCost)
	require.Equal(t, 8.8, repo.modelStats[0].AccountCost, "presentation view must not mutate raw model stats")

	groups, err := svc.GetGroupStatsWithFiltersForView(context.Background(), start, end, 0, 0, 0, 0, nil, nil, nil, true)
	require.NoError(t, err)
	require.Len(t, groups, 1)
	require.Zero(t, groups[0].AccountCost)
	require.Equal(t, 9.9, repo.groupStats[0].AccountCost, "presentation view must not mutate raw group stats")

	users, err := svc.GetUserBreakdownStatsForView(context.Background(), start, end, usagestats.UserBreakdownDimension{}, 10, true)
	require.NoError(t, err)
	require.Len(t, users, 1)
	require.Zero(t, users[0].AccountCost)
	require.Equal(t, 7.7, repo.userBreakdown[0].AccountCost, "presentation view must not mutate raw user breakdown")

	rawModels, err := svc.GetModelStatsWithFiltersBySourceForView(context.Background(), start, end, 0, 0, 0, 0, nil, nil, nil, usagestats.ModelSourceRequested, false)
	require.NoError(t, err)
	require.Equal(t, 8.8, rawModels[0].AccountCost)
}

func TestDashboardService_GetGroupUsageSummaryForViewUsesPresentation(t *testing.T) {
	repo := &usageRepoStub{
		groupSummary: []usagestats.GroupUsageSummary{{GroupID: 7, TodayCost: 2.5, TotalCost: 9.5}},
	}
	svc := NewDashboardService(repo, nil, nil, &config.Config{})

	got, err := svc.GetGroupUsageSummaryForView(context.Background(), time.Date(2026, 6, 22, 0, 0, 0, 0, time.UTC), UsageViewPresentation)

	require.NoError(t, err)
	require.True(t, repo.lastPresentation)
	require.Equal(t, []usagestats.GroupUsageSummary{{GroupID: 7, TodayCost: 2.5, TotalCost: 9.5}}, got)
}
