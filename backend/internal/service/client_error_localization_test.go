package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClientErrorMessageForAcceptLanguageLocalizesBillingAndTimeoutErrors(t *testing.T) {
	tests := []struct {
		name           string
		acceptLanguage string
		message        string
		want           string
	}{
		{
			name:           "chinese daily package quota",
			acceptLanguage: "zh-CN,zh;q=0.9",
			message:        "daily usage limit exceeded",
			want:           "当前套餐今日额度已用完，请在额度重置后重试",
		},
		{
			name:           "english total package quota",
			acceptLanguage: "en-US,en;q=0.9",
			message:        "total usage limit exceeded",
			want:           "The current subscription's total quota has been exhausted. Contact the administrator to renew or change the subscription.",
		},
		{
			name:           "missing language gets bilingual subscription error",
			acceptLanguage: "",
			message:        "No active subscription found for this group",
			want:           "No active subscription is available for this group. Contact the administrator to renew it. (当前分组没有有效订阅，请联系管理员续期)",
		},
		{
			name:           "english client translates legacy chinese api key error",
			acceptLanguage: "en",
			message:        "API key 额度已用完",
			want:           "The API key quota has been exhausted.",
		},
		{
			name:           "chinese upstream timeout",
			acceptLanguage: "zh",
			message:        "Upstream response timed out. Please retry later.",
			want:           "上游响应超时，请稍后重试",
		},
		{
			name:           "unknown upstream provider message stays unchanged",
			acceptLanguage: "zh",
			message:        "provider-specific diagnostic 9271",
			want:           "provider-specific diagnostic 9271",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, ClientErrorMessageForAcceptLanguage(tt.acceptLanguage, tt.message))
		})
	}
}

func TestClientErrorMessageForAcceptLanguageAppendsChineseHintForKnownUpstreamErrors(t *testing.T) {
	tests := []struct {
		name           string
		acceptLanguage string
		message        string
		want           string
	}{
		{
			name:           "model capacity keeps upstream diagnostic",
			acceptLanguage: "zh-CN,zh;q=0.9",
			message:        "Selected model is at capacity. Please try a different model.",
			want:           "Selected model is at capacity. Please try a different model. (中文：所选模型当前容量已满，请稍后重试或更换模型)",
		},
		{
			name:           "overload is bilingual even for english clients",
			acceptLanguage: "en-US,en;q=0.9",
			message:        "The server is overloaded. Please try again later.",
			want:           "The server is overloaded. Please try again later. (中文：上游服务当前过载，请稍后重试)",
		},
		{
			name:           "dynamic context message is recognized",
			acceptLanguage: "",
			message:        "Your input exceeds the maximum context length of this model.",
			want:           "Your input exceeds the maximum context length of this model. (中文：输入内容超过模型上下文长度限制，请缩短上下文后重试)",
		},
		{
			name:           "unsupported upstream capability has a chinese hint",
			acceptLanguage: "zh-CN",
			message:        "The requested capability is not supported by the upstream service. Use a supported model or feature. Upstream reason: Realtime is not enabled for this plan.",
			want:           "The requested capability is not supported by the upstream service. Use a supported model or feature. Upstream reason: Realtime is not enabled for this plan. (中文：上游暂不支持此模型或能力，请更换后重试)",
		},
		{
			name:           "upstream server failure has a chinese hint",
			acceptLanguage: "zh-CN",
			message:        "The upstream service is temporarily unavailable. Please retry later. Upstream reason: provider shard unavailable",
			want:           "The upstream service is temporarily unavailable. Please retry later. Upstream reason: provider shard unavailable (中文：上游服务内部错误，请稍后重试)",
		},
		{
			name:           "unknown upstream message remains unchanged",
			acceptLanguage: "zh-CN",
			message:        "provider-specific diagnostic 9271",
			want:           "provider-specific diagnostic 9271",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, ClientErrorMessageForAcceptLanguage(tt.acceptLanguage, tt.message))
		})
	}
}

func TestUpstreamFailureClientMessageClassifiesWithoutLeakingReason(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		body     string
		fallback string
		want     string
	}{
		{
			name:     "quota exhaustion",
			status:   429,
			body:     `{"error":{"code":"insufficient_quota","message":"You exceeded your current quota."}}`,
			fallback: "Upstream rate limit exceeded, please retry later",
			want:     "The upstream subscription quota has been exhausted. Please renew or wait for the quota to reset.",
		},
		{
			name:     "unsupported capability",
			status:   501,
			body:     `{"error":{"message":"Realtime is not enabled for this plan."}}`,
			fallback: "Upstream request failed",
			want:     "The requested model or capability is not supported by this upstream account. Choose another model or contact the administrator.",
		},
		{
			name:     "internal error",
			status:   500,
			body:     `{"error":{"message":"provider shard unavailable"}}`,
			fallback: "Upstream request failed",
			want:     "Upstream service temporarily unavailable",
		},
		{
			name:     "invalid request",
			status:   400,
			body:     `{"detail":"The reasoning effort is not supported by this model."}`,
			fallback: "Upstream request failed",
			want:     "The upstream service rejected the request. Check the model and request parameters, then retry.",
		},
		{
			name:     "authentication failure",
			status:   401,
			body:     `{"error":{"message":"API key has been revoked."}}`,
			fallback: "Upstream request failed",
			want:     "The upstream account authentication failed. Contact the administrator to refresh or replace the account.",
		},
		{
			name:     "authorization failure",
			status:   403,
			body:     `{"error":{"message":"This account cannot use batch processing."}}`,
			fallback: "Upstream request failed",
			want:     "The upstream account is not permitted to use this model or capability. Choose another model or contact the administrator.",
		},
		{
			name:     "missing model",
			status:   404,
			body:     `{"error":{"message":"The model gpt-example does not exist."}}`,
			fallback: "Upstream request failed",
			want:     "The requested model or resource is unavailable upstream. Check the model or choose another one.",
		},
		{
			name:     "request timeout",
			status:   408,
			body:     `{"error":{"message":"The provider did not receive the request in time."}}`,
			fallback: "Upstream request failed",
			want:     "Upstream response timed out. Please retry later.",
		},
		{
			name:     "request conflict",
			status:   409,
			body:     `{"error":{"message":"A batch with this idempotency key is already processing."}}`,
			fallback: "Upstream request failed",
			want:     "The upstream request conflicts with its current state. Retry later.",
		},
		{
			name:     "request too large",
			status:   413,
			body:     `{"error":{"message":"The input exceeds the provider size limit."}}`,
			fallback: "Upstream request failed",
			want:     "The request is too large for the upstream service. Reduce the input or attachments and retry.",
		},
		{
			name:     "invalid parameters",
			status:   422,
			body:     `{"error":{"message":"response_format is incompatible with this model."}}`,
			fallback: "Upstream request failed",
			want:     "The upstream service rejected the request parameters. Check the request and retry.",
		},
		{
			name:     "timeout",
			status:   504,
			body:     `{"message":"The provider timed out after 60 seconds."}`,
			fallback: "Upstream request failed",
			want:     "Upstream response timed out. Please retry later.",
		},
		{
			name:     "bad gateway",
			status:   502,
			body:     `{"error":{"message":"The provider proxy is unavailable."}}`,
			fallback: "Upstream request failed",
			want:     "Upstream service temporarily unavailable",
		},
		{
			name:     "service unavailable",
			status:   503,
			body:     `{"error":{"message":"The provider is in maintenance."}}`,
			fallback: "Upstream request failed",
			want:     "Upstream service temporarily unavailable",
		},
		{
			name:     "provider overload",
			status:   529,
			body:     `{"error":{"message":"The provider is overloaded."}}`,
			fallback: "Upstream request failed",
			want:     "Upstream service temporarily unavailable",
		},
		{
			name:     "unknown status hides reason",
			status:   418,
			body:     `{"error":{"message":"See https://provider.example/error?access_token=secret-value&reason=blocked"}}`,
			fallback: "Upstream request failed",
			want:     "Upstream request failed",
		},
		{
			name:     "spark image input",
			status:   400,
			body:     `{"error":{"message":"model gpt-5.3-codex-spark does not support image input"}}`,
			fallback: "Upstream request failed",
			want:     "The current model does not support image input. Remove image resources from the context, start a new conversation, or switch to a model that supports images.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := UpstreamFailureClientMessage(tt.status, []byte(tt.body), tt.fallback)
			require.Equal(t, tt.want, got)
			require.NotContains(t, got, "Upstream reason:")
			require.NotContains(t, got, "provider proxy is unavailable")
			require.NotContains(t, got, "You exceeded your current quota")
		})
	}
}
