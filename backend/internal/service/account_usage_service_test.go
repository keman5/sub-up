package service

import (
	"context"
	"net/http"
	"testing"
	"time"
)

type accountUsageCodexProbeRepo struct {
	stubOpenAIAccountRepo
	updateExtraCh chan map[string]any
	rateLimitCh   chan time.Time
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
