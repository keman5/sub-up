package handler

import (
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/config"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestResponsesQuotaFallbackUsesFinalForwardModelForChannelRestriction(t *testing.T) {
	for _, path := range []string{"/v1/responses", "/v1/responses/compact"} {
		t.Run(path, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			sourceID, destID, userID := int64(1001), int64(2001), int64(77)
			groups := &fallbackGroupRepo{groups: map[int64]*service.Group{
				sourceID: {ID: sourceID, Status: service.StatusActive, Platform: service.PlatformOpenAI, Hydrated: true, SubscriptionType: service.SubscriptionTypeSubscription, DailyLimitUSD: ptrFloat64(10), QuotaFallbackGroupID: &destID},
				destID:   {ID: destID, Status: service.StatusActive, Platform: service.PlatformOpenAI, Hydrated: true, SubscriptionType: service.SubscriptionTypeSubscription, DailyLimitUSD: ptrFloat64(100), ModelPolicyMode: service.GroupModelPolicyModeForce, ModelPolicyModel: "gpt-5.3-codex-spark"},
			}}
			sourceSub := &service.UserSubscription{ID: 11, UserID: userID, GroupID: sourceID, Status: service.SubscriptionStatusActive, ExpiresAt: time.Now().Add(time.Hour), DailyUsageUSD: 20}
			destSub := &service.UserSubscription{ID: 22, UserID: userID, GroupID: destID, Status: service.SubscriptionStatusActive, ExpiresAt: time.Now().Add(time.Hour), DailyUsageUSD: 1}
			subs := &fallbackSubRepo{subscriptions: map[string]*service.UserSubscription{fmt.Sprintf("%d:%d", userID, sourceID): sourceSub, fmt.Sprintf("%d:%d", userID, destID): destSub}}
			cfg := &config.Config{RunMode: config.RunModeStandard}
			cfg.Default.RateMultiplier = 1
			wireModel := "gpt-5.4"
			if strings.HasSuffix(path, "/compact") {
				wireModel = "gpt-5.4-compact"
			}
			channel := service.NewChannelService(&openAIWSUsageHandlerChannelRepoStub{
				channels: []service.Channel{
					{ID: 1, Status: service.StatusActive, GroupIDs: []int64{sourceID}, ModelMapping: map[string]map[string]string{service.PlatformOpenAI: {"gpt-5.5": "old-upstream"}}},
					{ID: 2, Status: service.StatusActive, GroupIDs: []int64{destID}, RestrictModels: true, BillingModelSource: service.BillingModelSourceUpstream, ModelMapping: map[string]map[string]string{service.PlatformOpenAI: {"gpt-5.3-codex-spark": "gpt-5.4"}}, ModelPricing: []service.ChannelModelPricing{{Platform: service.PlatformOpenAI, Models: []string{wireModel}, InputPrice: ptrFloat64(1), OutputPrice: ptrFloat64(1)}}},
				}, groupPlatforms: map[int64]string{sourceID: service.PlatformOpenAI, destID: service.PlatformOpenAI},
			}, nil, nil, nil, nil)
			accountRepo := &openAIWSFailoverHandlerAccountRepoStub{accounts: []service.Account{{ID: 9911, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Credentials: map[string]any{"api_key": "test", "base_url": "https://api.example.test"}, Extra: map[string]any{"openai_passthrough": true}}}}
			accountRepo.accounts[0].Credentials["compact_model_mapping"] = map[string]any{"gpt-5.4": "gpt-5.4-compact"}
			upstream := newAstraProCapturedUpstream(astra200())
			billing := service.NewBillingCacheService(nil, nil, subs, nil, nil, nil, cfg, nil)
			t.Cleanup(billing.Stop)
			gateway := service.NewOpenAIGatewayService(accountRepo, nil, nil, nil, nil, nil, nil, cfg, nil, nil, service.NewBillingService(cfg, nil), nil, billing, upstream, &service.DeferredService{}, nil, nil, nil, channel, nil, nil, nil)
			h := NewOpenAIGatewayHandler(gateway, service.NewConcurrencyService(nil), billing, service.NewAPIKeyService(nil, nil, nil, nil, nil, nil, cfg), nil, nil, nil, nil, cfg)
			h.subscriptionService = service.NewSubscriptionService(groups, subs, billing, nil, cfg)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"model":"gpt-5.5","input":"hello","stream":false}`))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{ID: 101, UserID: userID, User: &service.User{ID: userID, Status: service.StatusActive}, GroupID: &sourceID, Group: groups.groups[sourceID]})
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: userID})
			c.Set(string(middleware.ContextKeySubscription), sourceSub)
			h.Responses(c)
			_, ids, bodies := upstream.snapshot()
			require.Equal(t, []int64{9911}, ids, w.Body.String())
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			require.Len(t, bodies, 1)
			require.Equal(t, wireModel, gjson.GetBytes(bodies[0], "model").String(), "wire model and upstream restriction must agree after fallback")
		})
	}
}
