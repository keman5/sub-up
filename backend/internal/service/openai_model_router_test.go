package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestOpenAIModelRouterComplexTextDoesNotDoubleCountParsedBody(t *testing.T) {
	reqBody := openAIModelRouterTestBody(10, "short")
	rawBody, err := json.Marshal(reqBody)
	require.NoError(t, err)

	require.True(t, isOpenAIModelRouterComplexText(reqBody, rawBody, 0, 8))
	require.False(t, isOpenAIModelRouterComplexText(reqBody, rawBody, 0, 20))
}

func TestOpenAIModelRouterShortMultiItemRequestUsesDefaultTier(t *testing.T) {
	decision := decideOpenAIModelRouterTestRoute(t, openAIModelRouterTestBody(10, "short"), "auto-router", nil)

	require.True(t, decision.Enabled)
	require.Equal(t, openAIModelRouterTierEconomy, decision.Tier)
	require.Equal(t, "economy_default", decision.Reason)
	require.Equal(t, "gpt-5.3-codex-spark", decision.UpstreamModel)
}

func TestOpenAIModelRouterLongTextRequestUsesBalancedTier(t *testing.T) {
	decision := decideOpenAIModelRouterTestRoute(t, openAIModelRouterTestBody(1, strings.Repeat("x", 3000)), "auto-router", nil)

	require.True(t, decision.Enabled)
	require.Equal(t, openAIModelRouterTierBalanced, decision.Tier)
	require.Equal(t, "complex_text_promote_balanced", decision.Reason)
	require.Equal(t, "gpt-5.4", decision.UpstreamModel)
}

func TestOpenAIModelRouterPremiumTextUsesPremiumTier(t *testing.T) {
	decision := decideOpenAIModelRouterTestRoute(t, openAIModelRouterTestBody(1, strings.Repeat("x", 13000)), "auto-router", nil)

	require.True(t, decision.Enabled)
	require.Equal(t, openAIModelRouterTierPremium, decision.Tier)
	require.Equal(t, "premium_text_promote_premium", decision.Reason)
	require.Equal(t, "gpt-5.5", decision.UpstreamModel)
}

func TestOpenAIModelRouterExplicitGPT5UsesSparkWhenRemaining(t *testing.T) {
	decision := decideOpenAIModelRouterTestRoute(t, openAIModelRouterTestBody(1, "short"), "gpt-5.5", nil)

	require.True(t, decision.Enabled)
	require.Equal(t, openAIModelRouterTierEconomy, decision.Tier)
	require.Equal(t, "spark_remaining_over_threshold", decision.Reason)
	require.Equal(t, "gpt-5.3-codex-spark", decision.UpstreamModel)
}

func TestOpenAIModelRouterExplicitGPT5UsesRequestedWhenSparkLow(t *testing.T) {
	decision := decideOpenAIModelRouterTestRoute(t, openAIModelRouterTestBody(1, "short"), "gpt-5.5", map[string]any{
		"codex_5h_used_percent": 98.0,
		"codex_7d_used_percent": 98.0,
	})

	require.True(t, decision.Enabled)
	require.Equal(t, openAIModelRouterTierBalanced, decision.Tier)
	require.Equal(t, "spark_remaining_below_threshold_use_requested", decision.Reason)
	require.Equal(t, "gpt-5.5", decision.UpstreamModel)
}

func decideOpenAIModelRouterTestRoute(t *testing.T, reqBody map[string]any, requestedModel string, extra map[string]any) openAIModelRouterDecision {
	t.Helper()

	rawBody, err := json.Marshal(reqBody)
	require.NoError(t, err)
	if extra == nil {
		extra = map[string]any{
			"codex_5h_used_percent": 25.0,
			"codex_7d_used_percent": 16.0,
		}
	}

	svc := &OpenAIGatewayService{
		cfg: &config.Config{
			Gateway: config.GatewayConfig{
				ModelRouter: config.GatewayModelRouterConfig{
					Enabled:                        true,
					OAuthMode:                      "adaptive_codex",
					DefaultModel:                   "gpt-5.3-codex-spark",
					BalancedModel:                  "gpt-5.4",
					PremiumModel:                   "gpt-5.5",
					SessionRouteTTLSeconds:         1800,
					ComplexInputMinChars:           2400,
					ComplexInputMinItems:           8,
					PremiumInputMinChars:           12000,
					PremiumInputMinItems:           20,
					PressureLowRemainingPercent:    40,
					PressureMediumRemainingPercent: 70,
					ImageOrVisionForcePremium:      true,
				},
			},
		},
		rateLimitService: newOpenAIAdvancedSchedulerRateLimitService("true"),
	}
	t.Cleanup(resetOpenAIAdvancedSchedulerSettingCacheForTest)
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Extra:    extra,
	}

	return svc.decideOpenAIModelRoute(
		context.Background(),
		"session-1",
		account,
		reqBody,
		rawBody,
		requestedModel,
		false,
	)
}

func openAIModelRouterTestBody(inputCount int, text string) map[string]any {
	input := make([]any, inputCount)
	for i := range input {
		input[i] = map[string]any{
			"type": "message",
			"role": "user",
			"content": []any{
				map[string]any{
					"type": "input_text",
					"text": text,
				},
			},
		}
	}
	return map[string]any{
		"model": "gpt-5.5",
		"input": input,
	}
}
