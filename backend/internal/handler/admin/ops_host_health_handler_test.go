package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpsHandler_GetHostHealth(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "host-health.json")
	t.Setenv("SUB2API_HOST_HEALTH_PATH", path)
	require.NoError(t, os.WriteFile(path, []byte(`{
		"collected_at":"`+time.Now().UTC().Format(time.RFC3339)+`",
		"cpu":{"usage_percent":91.2,"high":true},
		"top_containers":[{"name":"sub2api-worker","cpu_percent":151.3,"memory":"800MiB / 1.172GiB","pids":18}]
	}`), 0o644))

	svc := service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	h := NewOpsHandler(svc)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/host-health", h.GetHostHealth)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/host-health", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var env responseEnvelope
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
	require.Equal(t, 0, env.Code)
	var snapshot service.OpsHostHealthSnapshot
	require.NoError(t, json.Unmarshal(env.Data, &snapshot))
	require.True(t, snapshot.Available)
	require.False(t, snapshot.Stale)
	require.True(t, snapshot.CPU.High)
	require.Len(t, snapshot.TopContainers, 1)
	require.Equal(t, "sub2api-worker", snapshot.TopContainers[0].Name)
}
