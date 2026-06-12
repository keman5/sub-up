package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestHeadroomStatsServiceDisabledByDefault(t *testing.T) {
	svc := NewHeadroomStatsService(&config.Config{})

	stats, err := svc.GetStats(context.Background())

	require.ErrorIs(t, err, ErrHeadroomStatsDisabled)
	require.Nil(t, stats)
}

func TestHeadroomStatsServiceFetchesTokenSavings(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/stats", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"summary": {
				"mode": "token",
				"api_requests": 12,
				"compression": {
					"requests_compressed": 3,
					"total_tokens_removed": 420,
					"avg_compression_pct": 12.5
				},
				"cost": {
					"total_saved_usd": 0.0042,
					"savings_pct": 9.1
				}
			},
			"requests": {
				"total": 12,
				"failed": 1,
				"by_provider": {"openai": 12},
				"by_model": {"gpt-5.5": 8, "gpt-5.4": 4}
			},
			"tokens": {
				"input": 1000,
				"output": 200,
				"saved": 420,
				"proxy_compression_saved": 400,
				"total_before_compression": 1420,
				"savings_percent": 29.577
			}
		}`))
	}))
	defer upstream.Close()

	svc := NewHeadroomStatsService(&config.Config{HeadroomStats: config.HeadroomStatsConfig{
		Enabled:        true,
		URL:            upstream.URL + "/stats",
		TimeoutSeconds: 2,
	}})

	stats, err := svc.GetStats(context.Background())

	require.NoError(t, err)
	require.NotNil(t, stats)
	require.Equal(t, "token", stats.Mode)
	require.Equal(t, int64(12), stats.APIRequests)
	require.Equal(t, int64(12), stats.RequestsTotal)
	require.Equal(t, int64(1), stats.RequestsFailed)
	require.Equal(t, int64(1000), stats.InputTokens)
	require.Equal(t, int64(200), stats.OutputTokens)
	require.Equal(t, int64(420), stats.TokensSaved)
	require.Equal(t, int64(400), stats.ProxyCompressionSaved)
	require.Equal(t, int64(1420), stats.TotalBeforeCompression)
	require.InDelta(t, 29.577, stats.SavingsPercent, 0.001)
	require.InDelta(t, 0.0042, stats.TotalSavedUSD, 0.0001)
	require.Equal(t, int64(8), stats.ByModel["gpt-5.5"])
	require.WithinDuration(t, time.Now(), stats.FetchedAt, 2*time.Second)
}
