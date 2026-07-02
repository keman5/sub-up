package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

type accountUsageCodexProbeRepo struct {
	stubOpenAIAccountRepo
	updateExtraCh chan map[string]any
	rateLimitCh   chan time.Time
	clearLimitCh  chan int64
	account       *Account
}

func (r *accountUsageCodexProbeRepo) UpdateExtra(_ context.Context, _ int64, updates map[string]any) error {
	if r.updateExtraCh != nil {
		copied := make(map[string]any, len(updates))
		for k, v := range updates {
			copied[k] = v
		}
		r.updateExtraCh <- copied
	}
	return nil
}

func (r *accountUsageCodexProbeRepo) SetRateLimited(_ context.Context, _ int64, resetAt time.Time) error {
	if r.rateLimitCh != nil {
		r.rateLimitCh <- resetAt
	}
	return nil
}

func (r *accountUsageCodexProbeRepo) ClearRateLimit(_ context.Context, id int64) error {
	if r.clearLimitCh != nil {
		r.clearLimitCh <- id
	}
	return nil
}

func (r *accountUsageCodexProbeRepo) GetByID(_ context.Context, id int64) (*Account, error) {
	if r.account != nil && r.account.ID == id {
		return r.account, nil
	}
	return r.stubOpenAIAccountRepo.GetByID(context.Background(), id)
}

type openAIQuotaUsageRefresherStub struct {
	calls int
	query func(context.Context, int64) (*OpenAIQuotaUsage, error)
}

func (s *openAIQuotaUsageRefresherStub) QueryUsage(ctx context.Context, accountID int64) (*OpenAIQuotaUsage, error) {
	s.calls++
	if s.query != nil {
		return s.query(ctx, accountID)
	}
	return &OpenAIQuotaUsage{}, nil
}

func TestShouldRefreshOpenAICodexSnapshot(t *testing.T) {
	t.Parallel()

	rateLimitedUntil := time.Now().Add(5 * time.Minute)
	now := time.Now()
	usage := &UsageInfo{
		FiveHour: &UsageProgress{Utilization: 0},
		SevenDay: &UsageProgress{Utilization: 0},
	}

	if !shouldRefreshOpenAICodexSnapshot(&Account{RateLimitResetAt: &rateLimitedUntil}, usage, now) {
		t.Fatal("expected rate-limited account to force codex snapshot refresh")
	}

	if shouldRefreshOpenAICodexSnapshot(&Account{}, usage, now) {
		t.Fatal("expected complete non-rate-limited usage to skip codex snapshot refresh")
	}

	if !shouldRefreshOpenAICodexSnapshot(&Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"openai_oauth_responses_websockets_v2_enabled": true,
			"codex_usage_updated_at":                       now.Format(time.RFC3339),
			"codex_main_usage_updated_at":                  now.Format(time.RFC3339),
			"codex_main_5h_used_percent":                   1.0,
			"codex_main_7d_used_percent":                   2.0,
		},
	}, usage, now) {
		t.Fatal("expected missing Spark snapshot to require refresh")
	}

	if !shouldRefreshOpenAICodexSnapshot(&Account{}, &UsageInfo{FiveHour: nil, SevenDay: &UsageProgress{}}, now) {
		t.Fatal("expected missing 5h snapshot to require refresh")
	}

	staleAt := now.Add(-(openAIProbeCacheTTL + time.Minute)).Format(time.RFC3339)
	if !shouldRefreshOpenAICodexSnapshot(&Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"openai_oauth_responses_websockets_v2_enabled": true,
			"codex_usage_updated_at":                       staleAt,
		},
	}, usage, now) {
		t.Fatal("expected stale ws snapshot to trigger refresh")
	}
}

// TestShouldRefreshOpenAICodexSnapshot_SparkShadowIgnoresWSv2 外审第9轮 P1:spark 影子用量走
// QueryUsage(/wham/usage,与 WSv2 无关),staleness 不得被 WSv2 门控,否则首刷后窗口永久冻结。
func TestShouldRefreshOpenAICodexSnapshot_SparkShadowIgnoresWSv2(t *testing.T) {
	t.Parallel()

	now := time.Now()
	usage := &UsageInfo{
		FiveHour: &UsageProgress{Utilization: 0},
		SevenDay: &UsageProgress{Utilization: 0},
	}
	staleAt := now.Add(-(openAIProbeCacheTTL + time.Minute)).Format(time.RFC3339)
	freshAt := now.Add(-time.Minute).Format(time.RFC3339)
	parentID := int64(7001)

	// 影子无 WSv2,但首刷后窗口已存在;过期 codex_usage_updated_at 必须触发再刷新。
	shadowStale := &Account{
		Platform:        PlatformOpenAI,
		Type:            AccountTypeOAuth,
		ParentAccountID: &parentID,
		QuotaDimension:  QuotaDimensionSpark,
		Extra:           map[string]any{"codex_usage_updated_at": staleAt},
	}
	if !shouldRefreshOpenAICodexSnapshot(shadowStale, usage, now) {
		t.Fatal("expected stale spark shadow (no WSv2) to trigger refresh")
	}

	// 影子时间戳仍新鲜→不刷(TTL 生效)。
	shadowFresh := &Account{
		Platform:        PlatformOpenAI,
		Type:            AccountTypeOAuth,
		ParentAccountID: &parentID,
		QuotaDimension:  QuotaDimensionSpark,
		Extra:           map[string]any{"codex_usage_updated_at": freshAt},
	}
	if shouldRefreshOpenAICodexSnapshot(shadowFresh, usage, now) {
		t.Fatal("expected fresh spark shadow to skip refresh (TTL not elapsed)")
	}

	// 反向对照:普通账号无 WSv2 + 过期时间戳→仍不刷(WSv2 门控普通账号的 probe 刷新)。
	normalNoWS := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Extra:    map[string]any{"codex_usage_updated_at": staleAt},
	}
	if shouldRefreshOpenAICodexSnapshot(normalNoWS, usage, now) {
		t.Fatal("expected non-WSv2 normal account to skip codex probe refresh")
	}
}

func TestExtractOpenAICodexProbeUpdatesAccepts429WithCodexHeaders(t *testing.T) {
	t.Parallel()

	headers := make(http.Header)
	headers.Set("x-codex-primary-used-percent", "100")
	headers.Set("x-codex-primary-reset-after-seconds", "604800")
	headers.Set("x-codex-primary-window-minutes", "10080")
	headers.Set("x-codex-secondary-used-percent", "100")
	headers.Set("x-codex-secondary-reset-after-seconds", "18000")
	headers.Set("x-codex-secondary-window-minutes", "300")

	updates, err := extractOpenAICodexProbeUpdates(&http.Response{StatusCode: http.StatusTooManyRequests, Header: headers})
	if err != nil {
		t.Fatalf("extractOpenAICodexProbeUpdates() error = %v", err)
	}
	if len(updates) == 0 {
		t.Fatal("expected codex probe updates from 429 headers")
	}
	if _, ok := updates["codex_main_5h_used_percent"]; ok {
		t.Fatalf("spark codex probe must not write main codex_main_5h_used_percent: %v", updates)
	}
	if got := updates["codex_5h_used_percent"]; got != 100.0 {
		t.Fatalf("codex_5h_used_percent = %v, want 100", got)
	}
	if got := updates["codex_7d_used_percent"]; got != 100.0 {
		t.Fatalf("codex_7d_used_percent = %v, want 100", got)
	}
}

func TestExtractOpenAICodexProbeUpdatesUsesActiveSparkLimitHeaders(t *testing.T) {
	t.Parallel()

	headers := make(http.Header)
	headers.Set("x-codex-active-limit", "codex_bengalfox")
	headers.Set("x-codex-primary-used-percent", "13")
	headers.Set("x-codex-primary-reset-after-seconds", "12701")
	headers.Set("x-codex-primary-window-minutes", "300")
	headers.Set("x-codex-secondary-used-percent", "29")
	headers.Set("x-codex-secondary-reset-after-seconds", "407818")
	headers.Set("x-codex-secondary-window-minutes", "10080")
	headers.Set("x-codex-bengalfox-limit-name", "GPT-5.3-Codex-Spark")
	headers.Set("x-codex-bengalfox-primary-used-percent", "0")
	headers.Set("x-codex-bengalfox-primary-reset-after-seconds", "18000")
	headers.Set("x-codex-bengalfox-primary-window-minutes", "300")
	headers.Set("x-codex-bengalfox-secondary-used-percent", "0")
	headers.Set("x-codex-bengalfox-secondary-reset-after-seconds", "586489")
	headers.Set("x-codex-bengalfox-secondary-window-minutes", "10080")

	updates, err := extractOpenAICodexProbeUpdatesForModel(&http.Response{StatusCode: http.StatusOK, Header: headers}, "gpt-5.3-codex-spark")
	if err != nil {
		t.Fatalf("extractOpenAICodexProbeUpdatesForModel() error = %v", err)
	}
	if got := updates["codex_5h_used_percent"]; got != 0.0 {
		t.Fatalf("codex_5h_used_percent = %v, want active Spark limit 0", got)
	}
	if got := updates["codex_7d_used_percent"]; got != 0.0 {
		t.Fatalf("codex_7d_used_percent = %v, want active Spark limit 0", got)
	}
	if got := updates["codex_primary_used_percent"]; got != 0.0 {
		t.Fatalf("raw spark primary = %v, want active Spark limit 0", got)
	}
	if got := updates["codex_secondary_used_percent"]; got != 0.0 {
		t.Fatalf("raw spark secondary = %v, want active Spark limit 0", got)
	}
}

func TestBuildOpenAICodexProbePayloadUsesSparkModel(t *testing.T) {
	t.Parallel()

	payload := buildOpenAICodexProbePayload()

	if got := payload["model"]; got != "gpt-5.3-codex-spark" {
		t.Fatalf("model = %v, want gpt-5.3-codex-spark", got)
	}
	if got := payload["store"]; got != false {
		t.Fatalf("store = %v, want false", got)
	}
}

func TestAccountUsageService_PersistOpenAICodexProbeSnapshotOnlyUpdatesExtra(t *testing.T) {
	t.Parallel()

	repo := &accountUsageCodexProbeRepo{
		updateExtraCh: make(chan map[string]any, 1),
		rateLimitCh:   make(chan time.Time, 1),
	}
	svc := &AccountUsageService{accountRepo: repo}
	svc.persistOpenAICodexProbeSnapshot(321, map[string]any{
		"codex_7d_used_percent": 100.0,
		"codex_7d_reset_at":     time.Now().Add(2 * time.Hour).UTC().Truncate(time.Second).Format(time.RFC3339),
	})

	select {
	case updates := <-repo.updateExtraCh:
		if got := updates["codex_7d_used_percent"]; got != 100.0 {
			t.Fatalf("codex_7d_used_percent = %v, want 100", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("等待 codex 探测快照写入 extra 超时")
	}

	select {
	case got := <-repo.rateLimitCh:
		t.Fatalf("不应将探测快照写入运行时限流状态: %v", got)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestAccountUsageService_GetOpenAIUsage_DoesNotPromoteCodexExtraToRateLimit(t *testing.T) {
	t.Parallel()

	resetAt := time.Now().Add(6 * 24 * time.Hour).UTC().Truncate(time.Second)
	repo := &accountUsageCodexProbeRepo{
		rateLimitCh: make(chan time.Time, 1),
	}
	svc := &AccountUsageService{accountRepo: repo}
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"codex_5h_used_percent": 1.0,
			"codex_5h_reset_at":     time.Now().Add(2 * time.Hour).UTC().Truncate(time.Second).Format(time.RFC3339),
			"codex_7d_used_percent": 100.0,
			"codex_7d_reset_at":     resetAt.Format(time.RFC3339),
		},
	}

	usage, err := svc.getOpenAIUsage(context.Background(), account, false)
	if err != nil {
		t.Fatalf("getOpenAIUsage() error = %v", err)
	}
	if usage.SevenDay != nil {
		t.Fatalf("不应将 Spark 7 天用量提升为主套餐 7 天用量: %#v", usage.SevenDay)
	}
	if usage.Codex7dUsedPercent == nil || *usage.Codex7dUsedPercent != 100.0 {
		t.Fatalf("预期 Spark 7 天用量仍然可见，实际为 %v", usage.Codex7dUsedPercent)
	}
	if account.RateLimitResetAt != nil {
		t.Fatalf("不应让已耗尽的 codex extra 改写运行时限流状态: %v", account.RateLimitResetAt)
	}
	select {
	case got := <-repo.rateLimitCh:
		t.Fatalf("不应将已耗尽的 codex extra 持久化为运行时限流状态: %v", got)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestAccountUsageService_GetOpenAIUsageSeparatesMainAndSparkSnapshots(t *testing.T) {
	t.Parallel()

	mainResetAt := time.Now().Add(2 * time.Hour).UTC().Truncate(time.Second).Format(time.RFC3339)
	sparkResetAt := time.Now().Add(90 * time.Minute).UTC().Truncate(time.Second).Format(time.RFC3339)
	svc := &AccountUsageService{}
	account := &Account{
		ID:       901,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"codex_main_5h_used_percent": 17.0,
			"codex_main_5h_reset_at":     mainResetAt,
			"codex_main_7d_used_percent": 41.0,
			"codex_main_7d_reset_at":     mainResetAt,
			"codex_5h_used_percent":      3.0,
			"codex_5h_reset_at":          sparkResetAt,
			"codex_7d_used_percent":      82.0,
			"codex_7d_reset_at":          sparkResetAt,
			"codex_usage_updated_at":     time.Now().Format(time.RFC3339),
		},
	}

	usage, err := svc.getOpenAIUsage(context.Background(), account, false)
	if err != nil {
		t.Fatalf("getOpenAIUsage() error = %v", err)
	}
	if usage.FiveHour == nil || usage.FiveHour.Utilization != 17.0 {
		t.Fatalf("main five_hour = %#v, want 17", usage.FiveHour)
	}
	if usage.CodexMain5hUsedPercent == nil || *usage.CodexMain5hUsedPercent != 17.0 {
		t.Fatalf("codex_main_5h_used_percent = %v, want 17", usage.CodexMain5hUsedPercent)
	}
	if usage.SevenDay == nil || usage.SevenDay.Utilization != 41.0 {
		t.Fatalf("main seven_day = %#v, want 41", usage.SevenDay)
	}
	if usage.CodexMain7dUsedPercent == nil || *usage.CodexMain7dUsedPercent != 41.0 {
		t.Fatalf("codex_main_7d_used_percent = %v, want 41", usage.CodexMain7dUsedPercent)
	}
	if usage.Codex5hUsedPercent == nil || *usage.Codex5hUsedPercent != 3.0 {
		t.Fatalf("spark codex_5h_used_percent = %v, want 3", usage.Codex5hUsedPercent)
	}
	if usage.Codex7dUsedPercent == nil || *usage.Codex7dUsedPercent != 82.0 {
		t.Fatalf("spark codex_7d_used_percent = %v, want 82", usage.Codex7dUsedPercent)
	}
}

func TestAccountUsageService_GetOpenAIUsageKeepsSparkSnapshotSeparateWithoutMain(t *testing.T) {
	t.Parallel()

	resetAt := time.Now().Add(2 * time.Hour).UTC().Truncate(time.Second).Format(time.RFC3339)
	svc := &AccountUsageService{}
	account := &Account{
		ID:       902,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"codex_5h_used_percent":          1.0,
			"codex_5h_reset_at":              resetAt,
			"codex_5h_window_minutes":        300,
			"codex_7d_used_percent":          22.0,
			"codex_7d_reset_at":              resetAt,
			"codex_7d_window_minutes":        10080,
			"codex_usage_updated_at":         time.Now().Format(time.RFC3339),
			"codex_primary_used_percent":     1.0,
			"codex_primary_window_minutes":   300,
			"codex_secondary_used_percent":   22.0,
			"codex_secondary_window_minutes": 10080,
		},
	}

	usage, err := svc.getOpenAIUsage(context.Background(), account, false)
	if err != nil {
		t.Fatalf("getOpenAIUsage() error = %v", err)
	}
	if usage.FiveHour != nil {
		t.Fatalf("spark snapshot without codex_main_* must not populate main five_hour: %#v", usage.FiveHour)
	}
	if usage.SevenDay != nil {
		t.Fatalf("spark snapshot without codex_main_* must not populate main seven_day: %#v", usage.SevenDay)
	}
	if usage.CodexMain5hUsedPercent != nil || usage.CodexMain7dUsedPercent != nil {
		t.Fatalf("spark snapshot without codex_main_* must not be exposed as main usage: 5h=%v 7d=%v", usage.CodexMain5hUsedPercent, usage.CodexMain7dUsedPercent)
	}
	if usage.Codex5hUsedPercent == nil || *usage.Codex5hUsedPercent != 1.0 {
		t.Fatalf("spark codex_5h_used_percent = %v, want 1", usage.Codex5hUsedPercent)
	}
	if usage.Codex7dUsedPercent == nil || *usage.Codex7dUsedPercent != 22.0 {
		t.Fatalf("spark codex_7d_used_percent = %v, want 22", usage.Codex7dUsedPercent)
	}
}

func TestAccountUsageService_GetOpenAIUsageMapsRawSparkSnapshotByWindowMinutes(t *testing.T) {
	t.Parallel()

	resetAt5h := time.Now().Add(2 * time.Hour).UTC().Truncate(time.Second).Format(time.RFC3339)
	resetAt7d := time.Now().Add(6 * 24 * time.Hour).UTC().Truncate(time.Second).Format(time.RFC3339)
	svc := &AccountUsageService{}
	account := &Account{
		ID:       903,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"codex_usage_updated_at":                     time.Now().Format(time.RFC3339),
			"codex_primary_used_percent":                 6.0,
			"codex_primary_window_minutes":               300,
			"codex_primary_reset_at":                     resetAt5h,
			"codex_primary_reset_after_seconds":          7200,
			"codex_secondary_used_percent":               42.0,
			"codex_secondary_window_minutes":             10080,
			"codex_secondary_reset_at":                   resetAt7d,
			"codex_secondary_reset_after_seconds":        518400,
			"codex_primary_over_secondary_limit_percent": 14.0,
		},
	}

	usage, err := svc.getOpenAIUsage(context.Background(), account, false)
	if err != nil {
		t.Fatalf("getOpenAIUsage() error = %v", err)
	}
	if usage.Codex5hUsedPercent == nil || *usage.Codex5hUsedPercent != 6.0 {
		t.Fatalf("codex_5h_used_percent = %v, want 6", usage.Codex5hUsedPercent)
	}
	if usage.Codex7dUsedPercent == nil || *usage.Codex7dUsedPercent != 42.0 {
		t.Fatalf("codex_7d_used_percent = %v, want 42", usage.Codex7dUsedPercent)
	}
	if usage.Codex5hResetAt == nil || *usage.Codex5hResetAt != resetAt5h {
		t.Fatalf("codex_5h_reset_at = %v, want %s", usage.Codex5hResetAt, resetAt5h)
	}
	if usage.Codex7dResetAt == nil || *usage.Codex7dResetAt != resetAt7d {
		t.Fatalf("codex_7d_reset_at = %v, want %s", usage.Codex7dResetAt, resetAt7d)
	}
}

func TestAccountUsageService_GetOpenAIUsageRefreshesOfficialQuotaForSuspiciousMainSnapshot(t *testing.T) {
	t.Parallel()

	resetAt := time.Now().Add(7 * time.Hour).UTC().Truncate(time.Second)
	refreshedAccount := &Account{
		ID:       905,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"codex_main_5h_used_percent": 0.0,
			"codex_main_5h_reset_at":     resetAt.Format(time.RFC3339),
			"codex_main_7d_used_percent": 0.0,
			"codex_main_7d_reset_at":     resetAt.Format(time.RFC3339),
			"codex_main_usage_updated_at": time.Now().
				UTC().
				Truncate(time.Second).
				Format(time.RFC3339),
		},
	}
	repo := &accountUsageCodexProbeRepo{account: refreshedAccount}
	refresher := &openAIQuotaUsageRefresherStub{}
	svc := &AccountUsageService{
		accountRepo:               repo,
		cache:                     NewUsageCache(),
		openAIQuotaUsageRefresher: refresher,
	}
	account := &Account{
		ID:               905,
		Platform:         PlatformOpenAI,
		Type:             AccountTypeOAuth,
		RateLimitResetAt: &resetAt,
		Extra: map[string]any{
			"codex_main_5h_used_percent": 100.0,
			"codex_main_5h_reset_at":     resetAt.Format(time.RFC3339),
			"codex_main_7d_used_percent": 100.0,
			"codex_main_7d_reset_at":     resetAt.Format(time.RFC3339),
		},
	}

	usage, err := svc.getOpenAIUsage(context.Background(), account, false)
	if err != nil {
		t.Fatalf("getOpenAIUsage() error = %v", err)
	}

	if refresher.calls != 1 {
		t.Fatalf("official quota refresh calls = %d, want 1", refresher.calls)
	}
	if usage.SevenDay == nil || usage.SevenDay.Utilization != 0 {
		t.Fatalf("seven day usage = %#v, want refreshed official main usage 0", usage.SevenDay)
	}
	if usage.CodexMain7dUsedPercent == nil || *usage.CodexMain7dUsedPercent != 0 {
		t.Fatalf("codex_main_7d_used_percent = %v, want 0", usage.CodexMain7dUsedPercent)
	}
}

func TestAccountUsageService_GetOpenAIUsageThrottlesOfficialQuotaRefresh(t *testing.T) {
	t.Parallel()

	resetAt := time.Now().Add(7 * time.Hour).UTC().Truncate(time.Second)
	refresher := &openAIQuotaUsageRefresherStub{}
	svc := &AccountUsageService{
		accountRepo:               &accountUsageCodexProbeRepo{},
		cache:                     NewUsageCache(),
		openAIQuotaUsageRefresher: refresher,
	}
	account := &Account{
		ID:       906,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"codex_main_7d_used_percent": 100.0,
			"codex_main_7d_reset_at":     resetAt.Format(time.RFC3339),
		},
	}

	for i := 0; i < 2; i++ {
		if _, err := svc.getOpenAIUsage(context.Background(), account, false); err != nil {
			t.Fatalf("getOpenAIUsage() call %d error = %v", i+1, err)
		}
	}

	if refresher.calls != 1 {
		t.Fatalf("official quota refresh calls = %d, want throttled single call", refresher.calls)
	}
}

func TestBuildCodexUsageProgressFromExtra_ZerosExpiredWindow(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC)

	t.Run("expired 5h window zeroes utilization", func(t *testing.T) {
		extra := map[string]any{
			"codex_5h_used_percent": 42.0,
			"codex_5h_reset_at":     "2026-03-16T10:00:00Z", // 2h ago
		}
		progress := buildCodexUsageProgressFromExtra(extra, "5h", now)
		if progress == nil {
			t.Fatal("expected non-nil progress")
		}
		if progress.Utilization != 0 {
			t.Fatalf("expected Utilization=0 for expired window, got %v", progress.Utilization)
		}
		if progress.RemainingSeconds != 0 {
			t.Fatalf("expected RemainingSeconds=0, got %v", progress.RemainingSeconds)
		}
	})

	t.Run("active 5h window keeps utilization", func(t *testing.T) {
		resetAt := now.Add(2 * time.Hour).Format(time.RFC3339)
		extra := map[string]any{
			"codex_5h_used_percent": 42.0,
			"codex_5h_reset_at":     resetAt,
		}
		progress := buildCodexUsageProgressFromExtra(extra, "5h", now)
		if progress == nil {
			t.Fatal("expected non-nil progress")
		}
		if progress.Utilization != 42.0 {
			t.Fatalf("expected Utilization=42, got %v", progress.Utilization)
		}
	})

	t.Run("expired 7d window zeroes utilization", func(t *testing.T) {
		extra := map[string]any{
			"codex_7d_used_percent": 88.0,
			"codex_7d_reset_at":     "2026-03-15T00:00:00Z", // yesterday
		}
		progress := buildCodexUsageProgressFromExtra(extra, "7d", now)
		if progress == nil {
			t.Fatal("expected non-nil progress")
		}
		if progress.Utilization != 0 {
			t.Fatalf("expected Utilization=0 for expired 7d window, got %v", progress.Utilization)
		}
	})
}

func TestAccountUsageService_GetOpenAIUsageZerosExpiredMainSnapshotFields(t *testing.T) {
	t.Parallel()

	resetAt := time.Now().Add(-time.Hour).UTC().Truncate(time.Second).Format(time.RFC3339)
	svc := &AccountUsageService{}
	account := &Account{
		ID:       904,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"codex_main_5h_used_percent": 100.0,
			"codex_main_5h_reset_at":     resetAt,
			"codex_main_7d_used_percent": 100.0,
			"codex_main_7d_reset_at":     resetAt,
		},
	}

	usage, err := svc.getOpenAIUsage(context.Background(), account, false)
	if err != nil {
		t.Fatalf("getOpenAIUsage() error = %v", err)
	}
	if usage.FiveHour == nil || usage.FiveHour.Utilization != 0 {
		t.Fatalf("expired main five_hour = %#v, want utilization 0", usage.FiveHour)
	}
	if usage.SevenDay == nil || usage.SevenDay.Utilization != 0 {
		t.Fatalf("expired main seven_day = %#v, want utilization 0", usage.SevenDay)
	}
	if usage.CodexMain5hUsedPercent == nil || *usage.CodexMain5hUsedPercent != 0 {
		t.Fatalf("codex_main_5h_used_percent = %v, want 0", usage.CodexMain5hUsedPercent)
	}
	if usage.CodexMain7dUsedPercent == nil || *usage.CodexMain7dUsedPercent != 0 {
		t.Fatalf("codex_main_7d_used_percent = %v, want 0", usage.CodexMain7dUsedPercent)
	}
}

func TestWindowStatsForViewHidesAccountCostForPresentation(t *testing.T) {
	stats := &WindowStats{
		Requests:     3,
		Tokens:       2400,
		Cost:         8,
		StandardCost: 10,
		UserCost:     4,
	}

	got := windowStatsForView(stats, UsageViewPresentation)

	if got == stats {
		t.Fatal("presentation view must return a copy")
	}
	if got.Requests != 3 || got.Tokens != 2400 {
		t.Fatalf("window counters = requests %d tokens %d, want requests 3 tokens 2400", got.Requests, got.Tokens)
	}
	if got.Cost != 4 || got.StandardCost != 4 || got.UserCost != 4 {
		t.Fatalf("presentation costs = cost %v standard %v user %v, want all 4", got.Cost, got.StandardCost, got.UserCost)
	}
	if stats.Cost != 8 || stats.StandardCost != 10 {
		t.Fatalf("presentation view mutated raw stats: cost %v standard %v", stats.Cost, stats.StandardCost)
	}
}

func TestWindowStatsForViewKeepsRawForSuperAdmin(t *testing.T) {
	stats := &WindowStats{
		Requests:     3,
		Tokens:       1200,
		Cost:         8,
		StandardCost: 10,
		UserCost:     4,
	}

	got := windowStatsForView(stats, UsageViewRaw)

	if got != stats {
		t.Fatal("raw view should return the original stats")
	}
	if got.Cost != 8 || got.StandardCost != 10 || got.UserCost != 4 {
		t.Fatalf("raw costs = cost %v standard %v user %v, want 8/10/4", got.Cost, got.StandardCost, got.UserCost)
	}
}

type accountWindowStatsForViewRepoStub struct {
	UsageLogRepository
	stats               *usagestats.AccountStats
	lastUsePresentation bool
}

func (r *accountWindowStatsForViewRepoStub) GetAccountTodayStatsForView(ctx context.Context, accountID int64, usePresentation bool) (*usagestats.AccountStats, error) {
	r.lastUsePresentation = usePresentation
	return r.stats, nil
}

func (r *accountWindowStatsForViewRepoStub) GetAccountWindowStatsForView(ctx context.Context, accountID int64, startTime time.Time, usePresentation bool) (*usagestats.AccountStats, error) {
	r.lastUsePresentation = usePresentation
	return r.stats, nil
}

func (r *accountWindowStatsForViewRepoStub) GetAccountWindowStatsBatchForView(ctx context.Context, accountIDs []int64, startTime time.Time, usePresentation bool) (map[int64]*usagestats.AccountStats, error) {
	r.lastUsePresentation = usePresentation
	result := make(map[int64]*usagestats.AccountStats, len(accountIDs))
	for _, accountID := range accountIDs {
		result[accountID] = r.stats
	}
	return result, nil
}

func TestAccountUsageServiceGetAccountWindowStatsForViewUsesPresentation(t *testing.T) {
	repo := &accountWindowStatsForViewRepoStub{
		stats: &usagestats.AccountStats{
			Requests:     2,
			Tokens:       3000,
			Cost:         8,
			StandardCost: 10,
			UserCost:     4,
		},
	}
	svc := &AccountUsageService{usageLogRepo: repo}

	got, err := svc.GetAccountWindowStatsForView(context.Background(), 7, time.Date(2026, 6, 22, 0, 0, 0, 0, time.UTC), UsageViewPresentation)

	if err != nil {
		t.Fatalf("GetAccountWindowStatsForView() error = %v", err)
	}
	if !repo.lastUsePresentation {
		t.Fatal("GetAccountWindowStatsForView() did not request presentation stats")
	}
	if got.Cost != 4 || got.StandardCost != 4 || got.UserCost != 4 {
		t.Fatalf("presentation window costs = cost %v standard %v user %v, want all 4", got.Cost, got.StandardCost, got.UserCost)
	}
	if got.Tokens != 3000 || got.Requests != 2 {
		t.Fatalf("presentation window counters = requests %d tokens %d, want 2/3000", got.Requests, got.Tokens)
	}
}
