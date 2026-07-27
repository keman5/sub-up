package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGatewayMiddlewareErrorWritersLocalizeMessages(t *testing.T) {
	tests := []struct {
		name   string
		write  func(*gin.Context)
		decode func(*testing.T, map[string]any) string
	}{
		{
			name: "standard auth envelope",
			write: func(c *gin.Context) {
				AbortWithError(c, http.StatusForbidden, "SUBSCRIPTION_NOT_FOUND", "No active subscription found for this group")
			},
			decode: func(t *testing.T, body map[string]any) string {
				t.Helper()
				message, ok := body["message"].(string)
				require.True(t, ok)
				return message
			},
		},
		{
			name: "anthropic envelope",
			write: func(c *gin.Context) {
				AnthropicErrorWriter(c, http.StatusForbidden, "No active subscription found for this group")
			},
			decode: func(t *testing.T, body map[string]any) string {
				t.Helper()
				errBody, ok := body["error"].(map[string]any)
				require.True(t, ok)
				message, ok := errBody["message"].(string)
				require.True(t, ok)
				return message
			},
		},
		{
			name: "google envelope",
			write: func(c *gin.Context) {
				GoogleErrorWriter(c, http.StatusForbidden, "No active subscription found for this group")
			},
			decode: func(t *testing.T, body map[string]any) string {
				t.Helper()
				errBody, ok := body["error"].(map[string]any)
				require.True(t, ok)
				message, ok := errBody["message"].(string)
				require.True(t, ok)
				return message
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
			c.Request.Header.Set("Accept-Language", "zh-CN")
			tt.write(c)

			var body map[string]any
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
			require.Equal(t, "当前分组没有有效订阅，请联系管理员续期", tt.decode(t, body))
		})
	}
}
