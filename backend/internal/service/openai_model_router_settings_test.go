package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestOpenAIGatewayService_EvaluateOpenAIModelRoute_RequiresGlobalSettingEnabled(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()

	svc := &OpenAIGatewayService{
		cfg: &config.Config{},
	}
	svc.cfg.Gateway.ModelRouter.Enabled = true
	svc.cfg.Gateway.ModelRouter.DefaultModel = "gpt-5.3-codex-spark"
	svc.cfg.Gateway.ModelRouter.BalancedModel = "gpt-5.4"
	svc.cfg.Gateway.ModelRouter.PremiumModel = "gpt-5.5"

	account := &Account{
		ID:       1,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Status:   StatusActive,
	}

	decision := svc.evaluateOpenAIModelRoute(context.Background(), openAIModelRouteEvalInput{
		SessionHash:    "session-disabled",
		Account:        account,
		RequestedModel: "gpt-5.5",
		RequestMeta: openAIModelRouterRequestMeta{
			RequestedModel: "gpt-5.5",
		},
	})

	require.False(t, decision.Enabled)
	require.Equal(t, openAIModelRouterTierEconomy, decision.Tier)
	require.Equal(t, "gpt-5.5", decision.UpstreamModel)
	require.Equal(t, "router_disabled", decision.Reason)
}

func TestOpenAIModelRouterCapImageIntentTier(t *testing.T) {
	require.Equal(
		t,
		openAIModelRouterTierEconomy,
		openAIModelRouterCapImageIntentTier(
			openAIModelRouterTierPremium,
			"gpt-5.3-codex-spark",
			"gpt-5.3-codex-spark",
			"gpt-5.4",
			"gpt-5.5",
		),
	)

	require.Equal(
		t,
		openAIModelRouterTierBalanced,
		openAIModelRouterCapImageIntentTier(
			openAIModelRouterTierPremium,
			"gpt-5.4",
			"gpt-5.3-codex-spark",
			"gpt-5.4",
			"gpt-5.5",
		),
	)

	require.Equal(
		t,
		openAIModelRouterTierPremium,
		openAIModelRouterCapImageIntentTier(
			openAIModelRouterTierPremium,
			"gpt-5.5",
			"gpt-5.3-codex-spark",
			"gpt-5.4",
			"gpt-5.5",
		),
	)
}
