package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestErrorFromLocalizesStructuredErrorsByAcceptLanguage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		acceptLanguage string
		err            error
		wantMessage    string
		wantReason     string
	}{
		{
			name:           "chinese_subscription_duplicate_confirmation",
			acceptLanguage: "zh-CN,zh;q=0.9",
			err:            infraerrors.Conflict("SUBSCRIPTION_DUPLICATE_CONFIRMATION_REQUIRED", "subscription already exists for this user and group; confirmation is required to replace it"),
			wantMessage:    "该用户已拥有此套餐订阅，确认后将覆盖旧订阅",
			wantReason:     "SUBSCRIPTION_DUPLICATE_CONFIRMATION_REQUIRED",
		},
		{
			name:           "english_troubleshooting_only",
			acceptLanguage: "en-US,en;q=0.9",
			err:            infraerrors.BadRequest("TROUBLESHOOTING_ONLY", "这里只能排查请求失败原因，请粘贴报错信息、状态码、接口 URL 或 request id"),
			wantMessage:    "This assistant can only diagnose request failures. Paste an error message, status code, API URL, or request id.",
			wantReason:     "TROUBLESHOOTING_ONLY",
		},
		{
			name:           "unknown_reason_keeps_original_message",
			acceptLanguage: "zh-CN",
			err:            infraerrors.BadRequest("SOME_NEW_REASON", "original message"),
			wantMessage:    "original message",
			wantReason:     "SOME_NEW_REASON",
		},
		{
			name:           "english_troubleshooting_notify_empty",
			acceptLanguage: "en-US,en;q=0.9",
			err:            infraerrors.BadRequest("TROUBLESHOOTING_NOTIFY_EMPTY", "缺少需要通知管理员的排查内容"),
			wantMessage:    "Troubleshooting details are required before notifying an administrator.",
			wantReason:     "TROUBLESHOOTING_NOTIFY_EMPTY",
		},
		{
			name:           "chinese_troubleshooting_notify_emails_empty",
			acceptLanguage: "zh-CN",
			err:            infraerrors.ServiceUnavailable("TROUBLESHOOTING_NOTIFY_EMAILS_EMPTY", "管理员通知邮箱未配置"),
			wantMessage:    "管理员通知邮箱未配置",
			wantReason:     "TROUBLESHOOTING_NOTIFY_EMAILS_EMPTY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
			c.Request.Header.Set("Accept-Language", tt.acceptLanguage)

			written := ErrorFrom(c, tt.err)

			require.True(t, written)
			var got Response
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
			require.Equal(t, tt.wantMessage, got.Message)
			require.Equal(t, tt.wantReason, got.Reason)
		})
	}
}

func TestErrorLocalizesCommonDirectHandlerMessagesByAcceptLanguage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		acceptLanguage string
		statusCode     int
		message        string
		wantMessage    string
	}{
		{
			name:           "chinese_invalid_subscription_id",
			acceptLanguage: "zh-CN,zh;q=0.9",
			statusCode:     http.StatusBadRequest,
			message:        "Invalid subscription ID",
			wantMessage:    "订阅 ID 无效",
		},
		{
			name:           "chinese_invalid_request_keeps_detail",
			acceptLanguage: "zh",
			statusCode:     http.StatusBadRequest,
			message:        "Invalid request: Key: 'AssignSubscriptionRequest.UserID' Error:Field validation for 'UserID' failed on the 'required' tag",
			wantMessage:    "请求参数无效：Key: 'AssignSubscriptionRequest.UserID' Error:Field validation for 'UserID' failed on the 'required' tag",
		},
		{
			name:           "chinese_unauthenticated",
			acceptLanguage: "zh-CN",
			statusCode:     http.StatusUnauthorized,
			message:        "User not authenticated",
			wantMessage:    "请先登录",
		},
		{
			name:           "chinese_image_generation_group_disabled",
			acceptLanguage: "zh-CN",
			statusCode:     http.StatusForbidden,
			message:        "Image generation is not enabled for this group",
			wantMessage:    "当前分组未启用图片生成",
		},
		{
			name:           "chinese_request_body_empty",
			acceptLanguage: "zh-CN",
			statusCode:     http.StatusBadRequest,
			message:        "Request body is empty",
			wantMessage:    "请求体不能为空",
		},
		{
			name:           "chinese_images_model_dynamic_message",
			acceptLanguage: "zh-CN",
			statusCode:     http.StatusBadRequest,
			message:        "images endpoint requires an image model, got \"gpt-5.3-codex-spark\"",
			wantMessage:    "图片接口需要使用图片模型，当前模型为 \"gpt-5.3-codex-spark\"",
		},
		{
			name:           "chinese_images_input_required",
			acceptLanguage: "zh-CN",
			statusCode:     http.StatusBadRequest,
			message:        "images[].image_url is required",
			wantMessage:    "images[].image_url 不能为空",
		},
		{
			name:           "english_keeps_direct_english_message",
			acceptLanguage: "en-US,en;q=0.9",
			statusCode:     http.StatusForbidden,
			message:        "Error requests view is disabled",
			wantMessage:    "Error requests view is disabled",
		},
		{
			name:        "missing_language_keeps_original",
			statusCode:  http.StatusBadRequest,
			message:     "Invalid subscription ID",
			wantMessage: "Invalid subscription ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.acceptLanguage != "" {
				c.Request.Header.Set("Accept-Language", tt.acceptLanguage)
			}

			Error(c, tt.statusCode, tt.message)

			var got Response
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
			require.Equal(t, tt.wantMessage, got.Message)
		})
	}
}
