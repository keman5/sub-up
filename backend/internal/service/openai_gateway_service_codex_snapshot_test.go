package service

import (
	"net/http"
	"testing"
	"time"
)

func TestCodexSnapshotBaseTime(t *testing.T) {
	fallback := time.Date(2026, 2, 20, 9, 0, 0, 0, time.UTC)

	t.Run("nil snapshot uses fallback", func(t *testing.T) {
		got := codexSnapshotBaseTime(nil, fallback)
		if !got.Equal(fallback) {
			t.Fatalf("got %v, want fallback %v", got, fallback)
		}
	})

	t.Run("empty updatedAt uses fallback", func(t *testing.T) {
		got := codexSnapshotBaseTime(&OpenAICodexUsageSnapshot{}, fallback)
		if !got.Equal(fallback) {
			t.Fatalf("got %v, want fallback %v", got, fallback)
		}
	})

	t.Run("valid updatedAt wins", func(t *testing.T) {
		got := codexSnapshotBaseTime(&OpenAICodexUsageSnapshot{UpdatedAt: "2026-02-16T10:00:00Z"}, fallback)
		want := time.Date(2026, 2, 16, 10, 0, 0, 0, time.UTC)
		if !got.Equal(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("invalid updatedAt uses fallback", func(t *testing.T) {
		got := codexSnapshotBaseTime(&OpenAICodexUsageSnapshot{UpdatedAt: "invalid"}, fallback)
		if !got.Equal(fallback) {
			t.Fatalf("got %v, want fallback %v", got, fallback)
		}
	})
}

func TestCodexResetAtRFC3339(t *testing.T) {
	base := time.Date(2026, 2, 16, 10, 0, 0, 0, time.UTC)

	t.Run("nil reset returns nil", func(t *testing.T) {
		if got := codexResetAtRFC3339(base, nil); got != nil {
			t.Fatalf("expected nil, got %v", *got)
		}
	})

	t.Run("positive seconds", func(t *testing.T) {
		sec := 90
		got := codexResetAtRFC3339(base, &sec)
		if got == nil {
			t.Fatal("expected non-nil")
		}
		if *got != "2026-02-16T10:01:30Z" {
			t.Fatalf("got %s, want %s", *got, "2026-02-16T10:01:30Z")
		}
	})

	t.Run("negative seconds clamp to base", func(t *testing.T) {
		sec := -3
		got := codexResetAtRFC3339(base, &sec)
		if got == nil {
			t.Fatal("expected non-nil")
		}
		if *got != "2026-02-16T10:00:00Z" {
			t.Fatalf("got %s, want %s", *got, "2026-02-16T10:00:00Z")
		}
	})
}

func TestBuildCodexUsageExtraUpdates_UsesSnapshotUpdatedAt(t *testing.T) {
	primaryUsed := 88.0
	primaryReset := 86400
	primaryWindow := 10080
	secondaryUsed := 12.0
	secondaryReset := 3600
	secondaryWindow := 300

	snapshot := &OpenAICodexUsageSnapshot{
		PrimaryUsedPercent:         &primaryUsed,
		PrimaryResetAfterSeconds:   &primaryReset,
		PrimaryWindowMinutes:       &primaryWindow,
		SecondaryUsedPercent:       &secondaryUsed,
		SecondaryResetAfterSeconds: &secondaryReset,
		SecondaryWindowMinutes:     &secondaryWindow,
		UpdatedAt:                  "2026-02-16T10:00:00Z",
	}

	updates := buildCodexUsageExtraUpdates(snapshot, time.Date(2026, 2, 20, 8, 0, 0, 0, time.UTC))
	if updates == nil {
		t.Fatal("expected non-nil updates")
	}

	if got := updates["codex_usage_updated_at"]; got != "2026-02-16T10:00:00Z" {
		t.Fatalf("codex_usage_updated_at = %v, want %s", got, "2026-02-16T10:00:00Z")
	}
	if got := updates["codex_5h_reset_at"]; got != "2026-02-16T11:00:00Z" {
		t.Fatalf("codex_5h_reset_at = %v, want %s", got, "2026-02-16T11:00:00Z")
	}
	if got := updates["codex_7d_reset_at"]; got != "2026-02-17T10:00:00Z" {
		t.Fatalf("codex_7d_reset_at = %v, want %s", got, "2026-02-17T10:00:00Z")
	}
}

// TestBuildCodexUsageExtraUpdates_FreshAccountUsedPercentNotInverted_Issue2994 locks in the
// canonical "used %" semantics for the 5h window. A fresh account reports a tiny
// secondary-used-percent (~1%); the stored codex_5h_used_percent must equal that value
// directly and must NOT be inverted to ~99%. Regression guard for issue #2994 / the reverted
// commit b65dde63 (PR #2918), which applied `100 - used` and made fresh accounts look
// exhausted, tripping auto-pause and excluding them from scheduling.
func TestBuildCodexUsageExtraUpdates_FreshAccountUsedPercentNotInverted_Issue2994(t *testing.T) {
	secondaryUsed := 1.0 // 5h window: barely used
	secondaryWindow := 300
	primaryUsed := 2.0 // 7d window: barely used
	primaryWindow := 10080

	snapshot := &OpenAICodexUsageSnapshot{
		PrimaryUsedPercent:     &primaryUsed,
		PrimaryWindowMinutes:   &primaryWindow,
		SecondaryUsedPercent:   &secondaryUsed,
		SecondaryWindowMinutes: &secondaryWindow,
		UpdatedAt:              "2026-02-16T10:00:00Z",
	}

	updates := buildCodexUsageExtraUpdates(snapshot, time.Date(2026, 2, 16, 10, 0, 0, 0, time.UTC))
	if updates == nil {
		t.Fatal("expected non-nil updates")
	}

	if got := updates["codex_5h_used_percent"]; got != 1.0 {
		t.Fatalf("codex_5h_used_percent = %v, want 1.0 (direct used%%, NOT inverted to 99)", got)
	}
	if got := updates["codex_7d_used_percent"]; got != 2.0 {
		t.Fatalf("codex_7d_used_percent = %v, want 2.0 (direct used%%, NOT inverted to 98)", got)
	}
}

func TestParseCodexRateLimitHeadersForModel_UsesActiveSparkLimitHeaders(t *testing.T) {
	headers := http.Header{}
	headers.Set("x-codex-active-limit", "codex_bengalfox")
	headers.Set("x-codex-primary-used-percent", "13")
	headers.Set("x-codex-primary-window-minutes", "300")
	headers.Set("x-codex-secondary-used-percent", "29")
	headers.Set("x-codex-secondary-window-minutes", "10080")
	headers.Set("x-codex-bengalfox-primary-used-percent", "0")
	headers.Set("x-codex-bengalfox-primary-window-minutes", "300")
	headers.Set("x-codex-bengalfox-secondary-used-percent", "0")
	headers.Set("x-codex-bengalfox-secondary-window-minutes", "10080")

	sparkSnapshot := ParseCodexRateLimitHeadersForModel(headers, "gpt-5.3-codex-spark")
	if sparkSnapshot == nil {
		t.Fatal("expected Spark snapshot")
	}
	sparkUpdates := buildCodexUsageExtraUpdatesForFamily(sparkSnapshot, time.Date(2026, 6, 13, 14, 0, 0, 0, time.UTC), "gpt-5.3-codex-spark")
	if got := sparkUpdates["codex_5h_used_percent"]; got != 0.0 {
		t.Fatalf("spark codex_5h_used_percent = %v, want active limit 0", got)
	}
	if got := sparkUpdates["codex_7d_used_percent"]; got != 0.0 {
		t.Fatalf("spark codex_7d_used_percent = %v, want active limit 0", got)
	}

	mainSnapshot := ParseCodexRateLimitHeadersForModel(headers, "gpt-5.3-codex")
	if mainSnapshot == nil {
		t.Fatal("expected main snapshot")
	}
	mainUpdates := buildCodexUsageExtraUpdatesForFamily(mainSnapshot, time.Date(2026, 6, 13, 14, 0, 0, 0, time.UTC), "gpt-5.3-codex")
	if got := mainUpdates["codex_main_5h_used_percent"]; got != 13.0 {
		t.Fatalf("main codex_main_5h_used_percent = %v, want generic 13", got)
	}
	if got := mainUpdates["codex_main_7d_used_percent"]; got != 29.0 {
		t.Fatalf("main codex_main_7d_used_percent = %v, want generic 29", got)
	}
}

func TestParseCodexRateLimitHeadersForModel_SparkFallsBackToGenericHeaders(t *testing.T) {
	headers := http.Header{}
	headers.Set("x-codex-primary-used-percent", "12")
	headers.Set("x-codex-primary-window-minutes", "300")
	headers.Set("x-codex-secondary-used-percent", "34")
	headers.Set("x-codex-secondary-window-minutes", "10080")

	snapshot := ParseCodexRateLimitHeadersForModel(headers, "gpt-5.3-codex-spark")
	if snapshot == nil {
		t.Fatal("expected fallback snapshot")
	}
	updates := buildCodexUsageExtraUpdatesForFamily(snapshot, time.Date(2026, 6, 13, 14, 0, 0, 0, time.UTC), "gpt-5.3-codex-spark")
	if got := updates["codex_5h_used_percent"]; got != 12.0 {
		t.Fatalf("codex_5h_used_percent = %v, want fallback generic 12", got)
	}
	if got := updates["codex_7d_used_percent"]; got != 34.0 {
		t.Fatalf("codex_7d_used_percent = %v, want fallback generic 34", got)
	}
}

func TestBuildCodexUsageExtraUpdatesForFamily_SeparatesMainFromSpark(t *testing.T) {
	primaryUsed := 73.0
	primaryWindow := 10080
	secondaryUsed := 11.0
	secondaryWindow := 300
	snapshot := &OpenAICodexUsageSnapshot{
		PrimaryUsedPercent:     &primaryUsed,
		PrimaryWindowMinutes:   &primaryWindow,
		SecondaryUsedPercent:   &secondaryUsed,
		SecondaryWindowMinutes: &secondaryWindow,
		UpdatedAt:              "2026-02-16T10:00:00Z",
	}

	mainUpdates := buildCodexUsageExtraUpdatesForFamily(snapshot, time.Date(2026, 2, 16, 10, 0, 0, 0, time.UTC), "gpt-5.3-codex")
	if mainUpdates == nil {
		t.Fatal("expected non-nil main updates")
	}
	if _, ok := mainUpdates["codex_5h_used_percent"]; ok {
		t.Fatalf("main model updates must not overwrite spark codex_5h_used_percent: %v", mainUpdates)
	}
	if _, ok := mainUpdates["codex_primary_used_percent"]; ok {
		t.Fatalf("main model updates must not overwrite raw spark primary snapshot: %v", mainUpdates)
	}
	if _, ok := mainUpdates["codex_secondary_used_percent"]; ok {
		t.Fatalf("main model updates must not overwrite raw spark secondary snapshot: %v", mainUpdates)
	}
	if got := mainUpdates["codex_main_5h_used_percent"]; got != 11.0 {
		t.Fatalf("codex_main_5h_used_percent = %v, want 11", got)
	}
	if got := mainUpdates["codex_main_7d_used_percent"]; got != 73.0 {
		t.Fatalf("codex_main_7d_used_percent = %v, want 73", got)
	}

	sparkUpdates := buildCodexUsageExtraUpdatesForFamily(snapshot, time.Date(2026, 2, 16, 10, 0, 0, 0, time.UTC), "gpt-5.3-codex-spark")
	if sparkUpdates == nil {
		t.Fatal("expected non-nil spark updates")
	}
	if _, ok := sparkUpdates["codex_main_5h_used_percent"]; ok {
		t.Fatalf("spark model updates must not overwrite main codex_main_5h_used_percent: %v", sparkUpdates)
	}
	if got := sparkUpdates["codex_5h_used_percent"]; got != 11.0 {
		t.Fatalf("codex_5h_used_percent = %v, want 11", got)
	}
	if got := sparkUpdates["codex_7d_used_percent"]; got != 73.0 {
		t.Fatalf("codex_7d_used_percent = %v, want 73", got)
	}
}

func TestBuildCodexUsageExtraUpdatesForFamily_WritesSparkFieldsForSparkModel(t *testing.T) {
	primaryUsed := 22.0
	primaryWindow := 10080
	secondaryUsed := 1.0
	secondaryWindow := 300
	snapshot := &OpenAICodexUsageSnapshot{
		PrimaryUsedPercent:     &primaryUsed,
		PrimaryWindowMinutes:   &primaryWindow,
		SecondaryUsedPercent:   &secondaryUsed,
		SecondaryWindowMinutes: &secondaryWindow,
		UpdatedAt:              "2026-06-13T08:06:05Z",
	}

	updates := buildCodexUsageExtraUpdatesForFamily(snapshot, time.Date(2026, 6, 13, 8, 0, 0, 0, time.UTC), "gpt-5.3-codex-spark")
	if updates == nil {
		t.Fatal("expected non-nil updates")
	}
	if _, ok := updates["codex_main_7d_used_percent"]; ok {
		t.Fatalf("spark model probe must not write main fields from x-codex headers: %v", updates)
	}
	if got := updates["codex_5h_used_percent"]; got != 1.0 {
		t.Fatalf("codex_5h_used_percent = %v, want 1", got)
	}
	if got := updates["codex_7d_used_percent"]; got != 22.0 {
		t.Fatalf("codex_7d_used_percent = %v, want 22", got)
	}
}

func TestBuildCodexUsageExtraUpdates_FallbackToNowWhenUpdatedAtInvalid(t *testing.T) {
	primaryUsed := 15.0
	primaryReset := 30
	primaryWindow := 300

	fallbackNow := time.Date(2026, 2, 20, 8, 30, 0, 0, time.UTC)
	snapshot := &OpenAICodexUsageSnapshot{
		PrimaryUsedPercent:       &primaryUsed,
		PrimaryResetAfterSeconds: &primaryReset,
		PrimaryWindowMinutes:     &primaryWindow,
		UpdatedAt:                "invalid-time",
	}

	updates := buildCodexUsageExtraUpdates(snapshot, fallbackNow)
	if updates == nil {
		t.Fatal("expected non-nil updates")
	}

	if got := updates["codex_usage_updated_at"]; got != "2026-02-20T08:30:00Z" {
		t.Fatalf("codex_usage_updated_at = %v, want %s", got, "2026-02-20T08:30:00Z")
	}
	if got := updates["codex_5h_reset_at"]; got != "2026-02-20T08:30:30Z" {
		t.Fatalf("codex_5h_reset_at = %v, want %s", got, "2026-02-20T08:30:30Z")
	}
}

func TestBuildCodexUsageExtraUpdates_ClampNegativeResetSeconds(t *testing.T) {
	primaryUsed := 90.0
	primaryReset := 7200
	primaryWindow := 10080
	secondaryUsed := 100.0
	secondaryReset := -15
	secondaryWindow := 300

	snapshot := &OpenAICodexUsageSnapshot{
		PrimaryUsedPercent:         &primaryUsed,
		PrimaryResetAfterSeconds:   &primaryReset,
		PrimaryWindowMinutes:       &primaryWindow,
		SecondaryUsedPercent:       &secondaryUsed,
		SecondaryResetAfterSeconds: &secondaryReset,
		SecondaryWindowMinutes:     &secondaryWindow,
		UpdatedAt:                  "2026-02-16T10:00:00Z",
	}

	updates := buildCodexUsageExtraUpdates(snapshot, time.Time{})
	if updates == nil {
		t.Fatal("expected non-nil updates")
	}

	if got := updates["codex_5h_reset_after_seconds"]; got != -15 {
		t.Fatalf("codex_5h_reset_after_seconds = %v, want %d", got, -15)
	}
	if got := updates["codex_5h_reset_at"]; got != "2026-02-16T10:00:00Z" {
		t.Fatalf("codex_5h_reset_at = %v, want %s", got, "2026-02-16T10:00:00Z")
	}
}

func TestBuildCodexUsageExtraUpdates_NilSnapshot(t *testing.T) {
	if got := buildCodexUsageExtraUpdates(nil, time.Now()); got != nil {
		t.Fatalf("expected nil updates, got %v", got)
	}
}

func TestBuildCodexUsageExtraUpdates_WithoutNormalizedWindowFields(t *testing.T) {
	primaryUsed := 42.0
	fallbackNow := time.Date(2026, 2, 20, 9, 15, 0, 0, time.UTC)
	snapshot := &OpenAICodexUsageSnapshot{
		PrimaryUsedPercent: &primaryUsed,
		UpdatedAt:          "",
	}

	updates := buildCodexUsageExtraUpdates(snapshot, fallbackNow)
	if updates == nil {
		t.Fatal("expected non-nil updates")
	}

	if got := updates["codex_usage_updated_at"]; got != "2026-02-20T09:15:00Z" {
		t.Fatalf("codex_usage_updated_at = %v, want %s", got, "2026-02-20T09:15:00Z")
	}
	if _, ok := updates["codex_5h_reset_at"]; ok {
		t.Fatalf("did not expect codex_5h_reset_at in updates: %v", updates["codex_5h_reset_at"])
	}
	if _, ok := updates["codex_7d_reset_at"]; ok {
		t.Fatalf("did not expect codex_7d_reset_at in updates: %v", updates["codex_7d_reset_at"])
	}
}
