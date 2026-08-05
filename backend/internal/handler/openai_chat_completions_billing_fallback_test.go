package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type fallbackGroupRepo struct {
	groups map[int64]*service.Group
}

func (r *fallbackGroupRepo) Create(context.Context, *service.Group) error { return nil }
func (r *fallbackGroupRepo) GetByID(ctx context.Context, id int64) (*service.Group, error) {
	group, ok := r.groups[id]
	if !ok {
		return nil, service.ErrGroupNotFound
	}
	cp := *group
	return &cp, nil
}
func (r *fallbackGroupRepo) GetByIDLite(_ context.Context, id int64) (*service.Group, error) {
	return r.GetByID(context.Background(), id)
}
func (r *fallbackGroupRepo) Update(context.Context, *service.Group) error          { return nil }
func (r *fallbackGroupRepo) Delete(context.Context, int64) error                   { return nil }
func (r *fallbackGroupRepo) DeleteCascade(context.Context, int64) ([]int64, error) { return nil, nil }
func (r *fallbackGroupRepo) List(context.Context, pagination.PaginationParams) ([]service.Group, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (r *fallbackGroupRepo) ListWithFilters(context.Context, pagination.PaginationParams, string, string, string, *bool) ([]service.Group, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (r *fallbackGroupRepo) ListActive(context.Context) ([]service.Group, error) { return nil, nil }
func (r *fallbackGroupRepo) ListActiveByPlatform(context.Context, string) ([]service.Group, error) {
	return nil, nil
}
func (r *fallbackGroupRepo) ExistsByName(context.Context, string) (bool, error) { return false, nil }
func (r *fallbackGroupRepo) GetAccountCount(context.Context, int64) (int64, int64, error) {
	return 0, 0, nil
}
func (r *fallbackGroupRepo) DeleteAccountGroupsByGroupID(context.Context, int64) (int64, error) {
	return 0, nil
}
func (r *fallbackGroupRepo) GetAccountIDsByGroupIDs(context.Context, []int64) ([]int64, error) {
	return nil, nil
}
func (r *fallbackGroupRepo) BindAccountsToGroup(context.Context, int64, []int64) error { return nil }
func (r *fallbackGroupRepo) UpdateSortOrders(context.Context, []service.GroupSortOrderUpdate) error {
	return nil
}

type fallbackSubRepo struct {
	subscriptions map[string]*service.UserSubscription
}

func (r *fallbackSubRepo) key(userID, groupID int64) string {
	return fmt.Sprintf("%d:%d", userID, groupID)
}

func (r *fallbackSubRepo) Create(ctx context.Context, sub *service.UserSubscription) error {
	if sub == nil {
		return nil
	}
	r.subscriptions[r.key(sub.UserID, sub.GroupID)] = sub
	return nil
}
func (r *fallbackSubRepo) GetByID(_ context.Context, id int64) (*service.UserSubscription, error) {
	for _, sub := range r.subscriptions {
		if sub != nil && sub.ID == id {
			cp := *sub
			return &cp, nil
		}
	}
	return nil, service.ErrSubscriptionNotFound
}
func (r *fallbackSubRepo) GetByIDForUpdate(ctx context.Context, id int64) (*service.UserSubscription, error) {
	return r.GetByID(ctx, id)
}
func (r *fallbackSubRepo) GetByIDIncludeDeleted(ctx context.Context, id int64) (*service.UserSubscription, error) {
	return r.GetByID(ctx, id)
}
func (r *fallbackSubRepo) GetByUserIDAndGroupID(ctx context.Context, userID, groupID int64) (*service.UserSubscription, error) {
	return r.GetActiveByUserIDAndGroupID(ctx, userID, groupID)
}
func (r *fallbackSubRepo) GetActiveByUserIDAndGroupID(_ context.Context, userID, groupID int64) (*service.UserSubscription, error) {
	sub, ok := r.subscriptions[r.key(userID, groupID)]
	if !ok || sub == nil {
		return nil, service.ErrSubscriptionNotFound
	}
	cp := *sub
	return &cp, nil
}
func (r *fallbackSubRepo) Update(context.Context, *service.UserSubscription) error { return nil }
func (r *fallbackSubRepo) Delete(context.Context, int64) error                     { return nil }
func (r *fallbackSubRepo) ListByUserID(context.Context, int64) ([]service.UserSubscription, error) {
	return nil, nil
}
func (r *fallbackSubRepo) ListActiveByUserID(context.Context, int64) ([]service.UserSubscription, error) {
	return nil, nil
}
func (r *fallbackSubRepo) ListByGroupID(context.Context, int64, pagination.PaginationParams) ([]service.UserSubscription, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (r *fallbackSubRepo) List(context.Context, pagination.PaginationParams, *int64, *int64, string, string, string, string) ([]service.UserSubscription, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (r *fallbackSubRepo) ExistsByUserIDAndGroupID(context.Context, int64, int64) (bool, error) {
	return false, nil
}
func (r *fallbackSubRepo) ExistsActiveByUserIDAndGroupID(ctx context.Context, userID, groupID int64) (bool, error) {
	_, err := r.GetActiveByUserIDAndGroupID(ctx, userID, groupID)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, service.ErrSubscriptionNotFound) {
		return false, nil
	}
	return false, err
}
func (r *fallbackSubRepo) Restore(ctx context.Context, id int64, restoredStatus string) (*service.UserSubscription, error) {
	sub, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	sub.Status = restoredStatus
	r.subscriptions[r.key(sub.UserID, sub.GroupID)] = sub
	return sub, nil
}
func (r *fallbackSubRepo) ExtendExpiry(context.Context, int64, time.Time) error    { return nil }
func (r *fallbackSubRepo) UpdateStatus(context.Context, int64, string) error       { return nil }
func (r *fallbackSubRepo) UpdateNotes(context.Context, int64, string) error        { return nil }
func (r *fallbackSubRepo) ActivateWindows(context.Context, int64, time.Time) error { return nil }
func (r *fallbackSubRepo) ResetUsageWindows(context.Context, int64, bool, bool, bool, time.Time) error {
	return nil
}
func (r *fallbackSubRepo) ResetDailyUsage(context.Context, int64, *time.Time, time.Time) error {
	return nil
}
func (r *fallbackSubRepo) ResetWeeklyUsage(context.Context, int64, *time.Time, time.Time) error {
	return nil
}
func (r *fallbackSubRepo) ResetMonthlyUsage(context.Context, int64, *time.Time, time.Time) error {
	return nil
}
func (r *fallbackSubRepo) IncrementUsage(context.Context, int64, float64) error { return nil }
func (r *fallbackSubRepo) BatchUpdateExpiredStatus(context.Context) ([]service.UserSubscription, error) {
	return nil, nil
}

func testOpenAIContext(method, path string) *gin.Context {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(method, path, nil)
	return c
}

func ptrFloat64(v float64) *float64 {
	return &v
}

func TestEnforceBillingEligibilityWithFallback_AppliesQuotaFallback(t *testing.T) {
	sourceGroupID := int64(1001)
	fallbackGroupID := int64(2001)
	userID := int64(77)

	groupRepo := &fallbackGroupRepo{groups: map[int64]*service.Group{}}
	groupRepo.groups[sourceGroupID] = &service.Group{
		ID:                   sourceGroupID,
		Status:               service.StatusActive,
		Platform:             service.PlatformOpenAI,
		Hydrated:             true,
		SubscriptionType:     service.SubscriptionTypeSubscription,
		DailyLimitUSD:        ptrFloat64(10),
		QuotaFallbackGroupID: &fallbackGroupID,
	}
	groupRepo.groups[fallbackGroupID] = &service.Group{
		ID:               fallbackGroupID,
		Status:           service.StatusActive,
		Platform:         service.PlatformOpenAI,
		Hydrated:         true,
		SubscriptionType: service.SubscriptionTypeSubscription,
		DailyLimitUSD:    ptrFloat64(100),
		ModelPolicyMode:  service.GroupModelPolicyModeForce,
		ModelPolicyModel: "gpt-5.3-codex-spark",
	}

	subscriptionRepo := &fallbackSubRepo{subscriptions: map[string]*service.UserSubscription{}}
	subscriptionRepo.subscriptions[fmt.Sprintf("%d:%d", userID, sourceGroupID)] = &service.UserSubscription{
		ID:            11,
		UserID:        userID,
		GroupID:       sourceGroupID,
		Status:        service.SubscriptionStatusActive,
		ExpiresAt:     time.Now().Add(time.Hour),
		DailyUsageUSD: 20,
	}
	subscriptionRepo.subscriptions[fmt.Sprintf("%d:%d", userID, fallbackGroupID)] = &service.UserSubscription{
		ID:            22,
		UserID:        userID,
		GroupID:       fallbackGroupID,
		Status:        service.SubscriptionStatusActive,
		ExpiresAt:     time.Now().Add(time.Hour),
		DailyUsageUSD: 1,
	}

	cfg := &config.Config{RunMode: config.RunModeStandard}
	billingCacheSvc := service.NewBillingCacheService(nil, nil, subscriptionRepo, nil, nil, nil, cfg, nil)
	subscriptionSvc := service.NewSubscriptionService(groupRepo, subscriptionRepo, billingCacheSvc, nil, cfg)

	apiKey := &service.APIKey{
		ID:      101,
		User:    &service.User{ID: userID},
		GroupID: &sourceGroupID,
		Group:   groupRepo.groups[sourceGroupID],
	}
	sourceSub := &service.UserSubscription{
		ID:            11,
		UserID:        userID,
		GroupID:       sourceGroupID,
		Status:        service.SubscriptionStatusActive,
		ExpiresAt:     time.Now().Add(time.Hour),
		DailyUsageUSD: 20,
	}

	h := &OpenAIGatewayHandler{
		billingCacheService: billingCacheSvc,
		subscriptionService: subscriptionSvc,
	}

	ctx := testOpenAIContext(http.MethodPost, "/openai/v1/chat/completions")

	calledHandle := false
	gotStatus := 0
	activeSub, activeAPIKey, ok := h.enforceBillingEligibilityWithFallback(
		ctx,
		apiKey,
		sourceSub,
		nil,
		nil,
		false,
		func(_ *gin.Context, status int, code, message string, _ bool) {
			calledHandle = true
			gotStatus = status
		},
	)

	require.True(t, ok)
	require.False(t, calledHandle)
	require.Equal(t, 0, gotStatus)
	require.NotNil(t, activeSub)
	require.Equal(t, int64(22), activeSub.ID)
	require.NotNil(t, activeAPIKey)
	require.Equal(t, fallbackGroupID, *activeAPIKey.GroupID)
	model, body := applyGroupModelPolicyToBody(activeAPIKey.Group, "gpt-5.5", []byte(`{"model":"gpt-5.5"}`))
	require.Equal(t, "gpt-5.3-codex-spark", model)
	require.JSONEq(t, `{"model":"gpt-5.3-codex-spark"}`, string(body))
	groupFromCtx, ok := ctx.Request.Context().Value(ctxkey.Group).(*service.Group)
	require.True(t, ok)
	require.Equal(t, fallbackGroupID, groupFromCtx.ID)
}

func TestEnforceBillingEligibilityWithFallback_WithoutUsableFallbackRejects(t *testing.T) {
	sourceGroupID := int64(1002)
	fallbackGroupID := int64(2002)
	userID := int64(88)

	groupRepo := &fallbackGroupRepo{groups: map[int64]*service.Group{}}
	groupRepo.groups[sourceGroupID] = &service.Group{
		ID:                   sourceGroupID,
		Status:               service.StatusActive,
		Platform:             service.PlatformOpenAI,
		Hydrated:             true,
		SubscriptionType:     service.SubscriptionTypeSubscription,
		DailyLimitUSD:        ptrFloat64(10),
		QuotaFallbackGroupID: &fallbackGroupID,
	}
	groupRepo.groups[fallbackGroupID] = &service.Group{
		ID:               fallbackGroupID,
		Status:           service.StatusActive,
		Platform:         service.PlatformOpenAI,
		Hydrated:         true,
		SubscriptionType: service.SubscriptionTypeSubscription,
		DailyLimitUSD:    ptrFloat64(100),
	}

	subscriptionRepo := &fallbackSubRepo{subscriptions: map[string]*service.UserSubscription{}}
	subscriptionRepo.subscriptions[fmt.Sprintf("%d:%d", userID, sourceGroupID)] = &service.UserSubscription{
		ID:            33,
		UserID:        userID,
		GroupID:       sourceGroupID,
		Status:        service.SubscriptionStatusActive,
		ExpiresAt:     time.Now().Add(time.Hour),
		DailyUsageUSD: 20,
	}
	subscriptionRepo.subscriptions[fmt.Sprintf("%d:%d", userID, fallbackGroupID)] = &service.UserSubscription{
		ID:            44,
		UserID:        userID,
		GroupID:       fallbackGroupID,
		Status:        service.SubscriptionStatusActive,
		ExpiresAt:     time.Now().Add(time.Hour),
		DailyUsageUSD: 200,
	}

	cfg := &config.Config{RunMode: config.RunModeStandard}
	billingCacheSvc := service.NewBillingCacheService(nil, nil, subscriptionRepo, nil, nil, nil, cfg, nil)
	subscriptionSvc := service.NewSubscriptionService(groupRepo, subscriptionRepo, billingCacheSvc, nil, cfg)

	apiKey := &service.APIKey{
		ID:      102,
		User:    &service.User{ID: userID},
		GroupID: &sourceGroupID,
		Group:   groupRepo.groups[sourceGroupID],
	}
	sourceSub := &service.UserSubscription{
		ID:            33,
		UserID:        userID,
		GroupID:       sourceGroupID,
		Status:        service.SubscriptionStatusActive,
		ExpiresAt:     time.Now().Add(time.Hour),
		DailyUsageUSD: 20,
	}

	h := &OpenAIGatewayHandler{
		billingCacheService: billingCacheSvc,
		subscriptionService: subscriptionSvc,
	}

	calledHandle := false
	gotStatus := 0
	gotCode := ""
	gotMessage := ""
	activeSub, activeAPIKey, ok := h.enforceBillingEligibilityWithFallback(
		testOpenAIContext(http.MethodPost, "/openai/v1/chat/completions"),
		apiKey,
		sourceSub,
		nil,
		nil,
		false,
		func(_ *gin.Context, status int, code, message string, _ bool) {
			calledHandle = true
			gotStatus = status
			gotCode = code
			gotMessage = message
		},
	)

	require.False(t, ok)
	require.True(t, calledHandle)
	require.Equal(t, http.StatusTooManyRequests, gotStatus)
	require.Equal(t, "rate_limit_exceeded", gotCode)
	require.Contains(t, gotMessage, "usage limit exceeded")
	require.Equal(t, sourceSub, activeSub)
	require.Equal(t, apiKey, activeAPIKey)
}
