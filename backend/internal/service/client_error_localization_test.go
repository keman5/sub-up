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
