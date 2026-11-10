package handler

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestChatQuotaFallbackUsesFinalForwardModel(t *testing.T) {
	for _, tc := range []struct {
		name           string
		relay, mapping bool
		want           string
	}{
		{"relay_forced_and_mapped", true, true, "gpt-5.4"},
		{"relay_forced_without_mapping", true, false, "gpt-5.3-codex-spark"},
		{"no_relay_mapped", false, true, "old-upstream"},
		{"no_relay_without_mapping", false, false, "gpt-5.5"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			sourceID, destID, userID := int64(1001), int64(2001), int64(77)
			groups := &fallbackGroupRepo{groups: map[int64]*service.Group{
				sourceID: {ID: sourceID, Status: service.StatusActive, Platform: service.PlatformOpenAI, Hydrated: true, SubscriptionType: service.SubscriptionTypeSubscription, DailyLimitUSD: ptrFloat64(10), QuotaFallbackGroupID: &destID},
				destID:   {ID: destID, Status: service.StatusActive, Platform: service.PlatformOpenAI, Hydrated: true, SubscriptionType: service.SubscriptionTypeSubscription, DailyLimitUSD: ptrFloat64(100), ModelPolicyMode: service.GroupModelPolicyModeForce, ModelPolicyModel: "gpt-5.3-codex-spark"},
			}}
			sourceSub := &service.UserSubscription{ID: 11, UserID: userID, GroupID: sourceID, Status: service.SubscriptionStatusActive, ExpiresAt: time.Now().Add(time.Hour), DailyUsageUSD: 1}
			if tc.relay {
				sourceSub.DailyUsageUSD = 20
			}
			destSub := &service.UserSubscription{ID: 22, UserID: userID, GroupID: destID, Status: service.SubscriptionStatusActive, ExpiresAt: time.Now().Add(time.Hour), DailyUsageUSD: 1}
			subs := &fallbackSubRepo{subscriptions: map[string]*service.UserSubscription{fmt.Sprintf("%d:%d", userID, sourceID): sourceSub, fmt.Sprintf("%d:%d", userID, destID): destSub}}
			cfg := &config.Config{RunMode: config.RunModeStandard}
			cfg.Default.RateMultiplier = 1
			channels := []service.Channel{
				{ID: 1, Status: service.StatusActive, GroupIDs: []int64{sourceID}},
				{ID: 2, Status: service.StatusActive, GroupIDs: []int64{destID}},
			}
			if tc.mapping {
				channels[0].ModelMapping = map[string]map[string]string{service.PlatformOpenAI: {"gpt-5.5": "old-upstream"}}
				channels[1].ModelMapping = map[string]map[string]string{service.PlatformOpenAI: {"gpt-5.3-codex-spark": "gpt-5.4"}}
			}
			// Only the final wire model is permitted. A stale scheduler model must
			// fail before transport, even if the forwarding body itself is correct.
			for i := range channels {
				channels[i].RestrictModels = true
				channels[i].BillingModelSource = service.BillingModelSourceUpstream
				channels[i].ModelPricing = []service.ChannelModelPricing{{Platform: service.PlatformOpenAI, Models: []string{tc.want}, InputPrice: ptrFloat64(1), OutputPrice: ptrFloat64(1)}}
			}
			channel := service.NewChannelService(&openAIWSUsageHandlerChannelRepoStub{channels: channels, groupPlatforms: map[int64]string{sourceID: service.PlatformOpenAI, destID: service.PlatformOpenAI}}, nil, nil, nil, nil)
			accountRepo := &openAIWSFailoverHandlerAccountRepoStub{accounts: []service.Account{{ID: 9911, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Credentials: map[string]any{"api_key": "test", "base_url": "https://api.example.test"}, Extra: map[string]any{"openai_passthrough": true, "openai_responses_supported": false}}}}
			upstream := newAstraProCapturedUpstream(&http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"id":"chatcmpl_test","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))})
			billing := service.NewBillingCacheService(nil, nil, subs, nil, nil, nil, cfg, nil)
			t.Cleanup(billing.Stop)
			gateway := service.NewOpenAIGatewayService(accountRepo, nil, nil, nil, nil, nil, nil, cfg, nil, nil, service.NewBillingService(cfg, nil), nil, billing, upstream, &service.DeferredService{}, nil, nil, nil, channel, nil, nil, nil)
			h := NewOpenAIGatewayHandler(gateway, service.NewConcurrencyService(nil), billing, service.NewAPIKeyService(nil, nil, nil, nil, nil, nil, cfg), nil, nil, nil, nil, cfg)
			h.subscriptionService = service.NewSubscriptionService(groups, subs, billing, nil, cfg)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-5.5","messages":[{"role":"user","content":"hello"}],"stream":false}`))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{ID: 101, UserID: userID, User: &service.User{ID: userID, Status: service.StatusActive}, GroupID: &sourceID, Group: groups.groups[sourceID]})
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: userID})
			c.Set(string(middleware.ContextKeySubscription), sourceSub)
			h.ChatCompletions(c)
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			urls, ids, bodies := upstream.snapshot()
			require.Equal(t, []int64{9911}, ids, "scheduler must permit the final model")
			require.Len(t, bodies, 1)
			require.Contains(t, urls[0], "/chat/completions")
			require.Equal(t, tc.want, gjson.GetBytes(bodies[0], "model").String(), "scheduler restriction and actual body must agree")
		})
	}
}
