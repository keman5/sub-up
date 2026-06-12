package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpsHandler_GetHeadroomStats(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"summary":{"mode":"token","api_requests":2,"compression":{"requests_compressed":1,"total_tokens_removed":10,"avg_compression_pct":5.5},"cost":{"total_saved_usd":0.0001,"savings_pct":2.5}},
			"requests":{"total":2,"failed":0,"by_provider":{"openai":2},"by_model":{"gpt-5.5":2}},
			"tokens":{"input":90,"output":10,"saved":10,"proxy_compression_saved":10,"total_before_compression":100,"savings_percent":10}
		}`))
	}))
	defer upstream.Close()

	svc := service.NewOpsService(nil, nil, &config.Config{
		HeadroomStats: config.HeadroomStatsConfig{
			Enabled:        true,
			URL:            upstream.URL,
			TimeoutSeconds: 2,
		},
	}, nil, nil, nil, nil, nil, nil, nil, nil)
	h := NewOpsHandler(svc)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/headroom/stats", h.GetHeadroomStats)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/headroom/stats", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var env responseEnvelope
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
	require.Equal(t, 0, env.Code)
	var stats service.HeadroomStatsSnapshot
	require.NoError(t, json.Unmarshal(env.Data, &stats))
	require.Equal(t, int64(10), stats.TokensSaved)
	require.Equal(t, int64(100), stats.TotalBeforeCompression)
	require.Equal(t, int64(2), stats.ByProvider["openai"])
}

func TestOpsHandler_GetHeadroomStatsDisabled(t *testing.T) {
	svc := service.NewOpsService(nil, nil, &config.Config{}, nil, nil, nil, nil, nil, nil, nil, nil)
	h := NewOpsHandler(svc)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/headroom/stats", h.GetHeadroomStats)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/headroom/stats", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusServiceUnavailable, w.Code)
}
