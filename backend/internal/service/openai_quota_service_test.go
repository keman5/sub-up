package service

import (
	"context"
	"testing"
	"time"
)

func TestOpenAIQuotaServiceSyncQuotaUsageSnapshotWritesMainAndSpark(t *testing.T) {
	t.Parallel()

	repo := &accountUsageCodexProbeRepo{updateExtraCh: make(chan map[string]any, 1)}
	svc := &OpenAIQuotaService{accountRepo: repo}
	now := time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC)

	svc.syncQuotaUsageSnapshot(context.Background(), 901, &OpenAIQuotaUsage{
		RateLimit: &OpenAIRateLimit{
			PrimaryWindow: &OpenAIRateLimitWindow{
				UsedPercent:        0,
				LimitWindowSeconds: 7 * 24 * 60 * 60,
				ResetAfterSeconds:  7 * 24 * 60 * 60,
			},
			SecondaryWindow: &OpenAIRateLimitWindow{
				UsedPercent:        0,
				LimitWindowSeconds: 5 * 60 * 60,
				ResetAfterSeconds:  5 * 60 * 60,
			},
		},
		AdditionalRateLimits: []OpenAIAdditionalRateLimit{
			{
				LimitName:      "GPT-5.3-Codex-Spark",
				MeteredFeature: "codex_bengalfox",
				RateLimit: &OpenAIRateLimit{
					PrimaryWindow: &OpenAIRateLimitWindow{
						UsedPercent:        12,
						LimitWindowSeconds: 7 * 24 * 60 * 60,
						ResetAfterSeconds:  6 * 24 * 60 * 60,
					},
					SecondaryWindow: &OpenAIRateLimitWindow{
						UsedPercent:        3,
						LimitWindowSeconds: 5 * 60 * 60,
						ResetAfterSeconds:  4 * 60 * 60,
					},
				},
			},
		},
	}, now)

	select {
	case updates := <-repo.updateExtraCh:
		if got := updates["codex_main_7d_used_percent"]; got != 0.0 {
			t.Fatalf("codex_main_7d_used_percent = %v, want 0", got)
		}
		if got := updates["codex_main_5h_used_percent"]; got != 0.0 {
			t.Fatalf("codex_main_5h_used_percent = %v, want 0", got)
		}
		if got := updates["codex_7d_used_percent"]; got != 12.0 {
			t.Fatalf("codex_7d_used_percent = %v, want 12", got)
		}
		if got := updates["codex_5h_used_percent"]; got != 3.0 {
			t.Fatalf("codex_5h_used_percent = %v, want 3", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("waiting for quota snapshot sync timed out")
	}
}

func TestOpenAIQuotaServiceClearLocalRateLimitIfQuotaRecovered(t *testing.T) {
	t.Parallel()

	repo := &accountUsageCodexProbeRepo{clearLimitCh: make(chan int64, 1)}
	svc := &OpenAIQuotaService{accountRepo: repo}

	svc.clearLocalRateLimitIfQuotaRecovered(context.Background(), 901, &OpenAIQuotaUsage{
		RateLimit: &OpenAIRateLimit{
			Allowed:      true,
			LimitReached: false,
			PrimaryWindow: &OpenAIRateLimitWindow{
				UsedPercent:        0,
				LimitWindowSeconds: 7 * 24 * 60 * 60,
				ResetAfterSeconds:  7 * 24 * 60 * 60,
			},
		},
	})

	select {
	case id := <-repo.clearLimitCh:
		if id != 901 {
			t.Fatalf("cleared account id = %d, want 901", id)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected recovered quota to clear local rate limit")
	}
}

func TestOpenAIQuotaServiceDoesNotClearLocalRateLimitWhenQuotaStillReached(t *testing.T) {
	t.Parallel()

	repo := &accountUsageCodexProbeRepo{clearLimitCh: make(chan int64, 1)}
	svc := &OpenAIQuotaService{accountRepo: repo}

	svc.clearLocalRateLimitIfQuotaRecovered(context.Background(), 901, &OpenAIQuotaUsage{
		RateLimit: &OpenAIRateLimit{
			Allowed:      false,
			LimitReached: true,
			PrimaryWindow: &OpenAIRateLimitWindow{
				UsedPercent:        100,
				LimitWindowSeconds: 7 * 24 * 60 * 60,
				ResetAfterSeconds:  7 * 60 * 60,
			},
		},
	})

	select {
	case id := <-repo.clearLimitCh:
		t.Fatalf("unexpected local rate limit clear for account %d", id)
	case <-time.After(200 * time.Millisecond):
	}
}
