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
			name:           "unknown_reason_keeps_original_message",
			acceptLanguage: "zh-CN",
			err:            infraerrors.BadRequest("SOME_NEW_REASON", "original message"),
			wantMessage:    "original message",
			wantReason:     "SOME_NEW_REASON",
		},
		{
			name:           "chinese_api_key_expired",
			acceptLanguage: "zh-CN",
			err:            infraerrors.Unauthorized("API_KEY_EXPIRED", "api key has expired"),
			wantMessage:    "API Key 已过期",
			wantReason:     "API_KEY_EXPIRED",
		},
		{
			name:           "chinese_balance_not_enough",
			acceptLanguage: "zh-CN",
			err:            infraerrors.Forbidden("BALANCE_NOT_ENOUGH", "balance is not enough"),
			wantMessage:    "余额不足",
			wantReason:     "BALANCE_NOT_ENOUGH",
		},
		{
			name:           "english_group_not_found",
			acceptLanguage: "en-US,en;q=0.9",
			err:            infraerrors.NotFound("GROUP_NOT_FOUND", "group not found"),
			wantMessage:    "Group not found.",
			wantReason:     "GROUP_NOT_FOUND",
		},
		{
			name:           "chinese_oauth_email_not_verified",
			acceptLanguage: "zh-CN",
			err:            infraerrors.Forbidden("OAUTH_EMAIL_NOT_VERIFIED", "oauth email is not verified"),
			wantMessage:    "第三方账号邮箱尚未验证",
			wantReason:     "OAUTH_EMAIL_NOT_VERIFIED",
		},
		{
			name:           "chinese_proxy_in_use",
			acceptLanguage: "zh-CN",
			err:            infraerrors.Conflict("PROXY_IN_USE", "proxy is in use"),
			wantMessage:    "代理正在使用中，不能删除",
			wantReason:     "PROXY_IN_USE",
		},
		{
			name:           "english_ops_disabled",
			acceptLanguage: "en-US,en;q=0.9",
			err:            infraerrors.NotFound("OPS_DISABLED", "Ops monitoring is disabled"),
			wantMessage:    "Operations monitoring is disabled.",
			wantReason:     "OPS_DISABLED",
		},
		{
			name:           "chinese_refresh_token_expired",
			acceptLanguage: "zh-CN",
			err:            infraerrors.Unauthorized("REFRESH_TOKEN_EXPIRED", "refresh token expired"),
			wantMessage:    "登录状态已过期，请重新登录",
			wantReason:     "REFRESH_TOKEN_EXPIRED",
		},
		{
			name:           "chinese_registration_disabled",
			acceptLanguage: "zh-CN",
			err:            infraerrors.Forbidden("REGISTRATION_DISABLED", "registration is currently disabled"),
			wantMessage:    "注册功能暂未开放",
			wantReason:     "REGISTRATION_DISABLED",
		},
		{
			name:           "english_promo_code_expired",
			acceptLanguage: "en-US,en;q=0.9",
			err:            infraerrors.BadRequest("PROMO_CODE_EXPIRED", "promo code expired"),
			wantMessage:    "Promo code has expired.",
			wantReason:     "PROMO_CODE_EXPIRED",
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
		{
			name:           "chinese_setup_already_installed",
			acceptLanguage: "zh-CN",
			statusCode:     http.StatusForbidden,
			message:        "Setup is not allowed: system is already installed",
			wantMessage:    "系统已安装，不能再次执行初始化",
		},
		{
			name:           "chinese_sync_upstream_models_failed",
			acceptLanguage: "zh-CN",
			statusCode:     http.StatusBadGateway,
			message:        "Failed to sync upstream models from upstream",
			wantMessage:    "从上游同步模型失败",
		},
		{
			name:           "chinese_usage_cleanup_unavailable",
			acceptLanguage: "zh-CN",
			statusCode:     http.StatusServiceUnavailable,
			message:        "Usage cleanup service unavailable",
			wantMessage:    "用量清理服务暂时不可用",
		},
		{
			name:           "chinese_invalid_hostname_format",
			acceptLanguage: "zh-CN",
			statusCode:     http.StatusBadRequest,
			message:        "Invalid hostname format",
			wantMessage:    "主机名格式无效",
		},
		{
			name:           "chinese_platform_quota_service_unavailable",
			acceptLanguage: "zh-CN",
			statusCode:     http.StatusServiceUnavailable,
			message:        "platform quota service not available",
			wantMessage:    "平台额度服务暂时不可用",
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
