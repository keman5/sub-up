package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newLocalizedGatewayTestContext(t *testing.T, acceptLanguage string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Request.Header.Set("Accept-Language", acceptLanguage)
	return c, recorder
}

func decodeGatewayErrorMessage(t *testing.T, recorder *httptest.ResponseRecorder, path ...string) string {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	current := any(body)
	for _, key := range path {
		object, ok := current.(map[string]any)
		require.True(t, ok)
		current = object[key]
	}
	message, ok := current.(string)
	require.True(t, ok)
	return message
}

func TestGatewayProtocolErrorsLocalizePackageQuota(t *testing.T) {
	t.Run("chat completions", func(t *testing.T) {
		c, recorder := newLocalizedGatewayTestContext(t, "zh-CN")
		(&GatewayHandler{}).chatCompletionsErrorResponse(c, http.StatusTooManyRequests, "rate_limit_exceeded", "daily usage limit exceeded")
		require.Equal(t, "当前套餐今日额度已用完，请在额度重置后重试", decodeGatewayErrorMessage(t, recorder, "error", "message"))
	})

	t.Run("responses", func(t *testing.T) {
		c, recorder := newLocalizedGatewayTestContext(t, "zh-CN")
		(&GatewayHandler{}).responsesErrorResponse(c, http.StatusTooManyRequests, "rate_limit_exceeded", "weekly usage limit exceeded")
		require.Equal(t, "当前套餐本周额度已用完，请在额度重置后重试", decodeGatewayErrorMessage(t, recorder, "error", "message"))
	})

	t.Run("anthropic openai gateway", func(t *testing.T) {
		c, recorder := newLocalizedGatewayTestContext(t, "zh-CN")
		(&OpenAIGatewayHandler{}).anthropicErrorResponse(c, http.StatusTooManyRequests, "rate_limit_exceeded", "monthly usage limit exceeded")
		require.Equal(t, "当前套餐本月额度已用完，请在额度重置后重试", decodeGatewayErrorMessage(t, recorder, "error", "message"))
	})

	t.Run("stream terminal error", func(t *testing.T) {
		c, recorder := newLocalizedGatewayTestContext(t, "zh-CN")
		(&GatewayHandler{}).handleStreamingAwareError(c, http.StatusGatewayTimeout, "upstream_timeout", "Upstream response timed out. Please retry later.", true)
		require.Contains(t, recorder.Body.String(), "上游响应超时，请稍后重试")
		require.False(t, strings.Contains(recorder.Body.String(), "Upstream response timed out"))
	})

	t.Run("openai stream terminal error", func(t *testing.T) {
		c, recorder := newLocalizedGatewayTestContext(t, "zh-CN")
		(&OpenAIGatewayHandler{}).handleStreamingAwareError(c, http.StatusGatewayTimeout, "upstream_timeout", "Upstream response timed out. Please retry later.", true)
		require.Contains(t, recorder.Body.String(), "上游响应超时，请稍后重试")
		require.False(t, strings.Contains(recorder.Body.String(), "Upstream response timed out"))
	})
}

func TestGatewayUpstreamTimeoutMappingPreserves504(t *testing.T) {
	tests := []struct {
		name     string
		mapError func(int) (int, string, string)
	}{
		{name: "anthropic gateway", mapError: (&GatewayHandler{}).mapUpstreamError},
		{name: "openai gateway", mapError: (&OpenAIGatewayHandler{}).mapUpstreamError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, code, message := tt.mapError(http.StatusGatewayTimeout)
			require.Equal(t, http.StatusGatewayTimeout, status)
			require.Equal(t, "upstream_timeout", code)
			require.Equal(t, "Upstream response timed out. Please retry later.", message)
		})
	}
}
