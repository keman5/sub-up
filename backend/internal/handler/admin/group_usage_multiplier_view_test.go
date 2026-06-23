package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type groupUsageMultiplierAdminServiceStub struct {
	service.AdminService
	createInput *service.CreateGroupInput
	updateInput *service.UpdateGroupInput
}

func (s *groupUsageMultiplierAdminServiceStub) CreateGroup(ctx context.Context, input *service.CreateGroupInput) (*service.Group, error) {
	s.createInput = input
	return &service.Group{
		ID:                     11,
		Name:                   input.Name,
		Platform:               input.Platform,
		RateMultiplier:         input.RateMultiplier,
		UsageMultiplierEnabled: true,
		UsageMultiplier:        2,
		Status:                 service.StatusActive,
		SubscriptionType:       service.SubscriptionTypeSubscription,
	}, nil
}

func (s *groupUsageMultiplierAdminServiceStub) UpdateGroup(ctx context.Context, id int64, input *service.UpdateGroupInput) (*service.Group, error) {
	s.updateInput = input
	return &service.Group{
		ID:                     id,
		Name:                   "updated",
		Platform:               service.PlatformAnthropic,
		RateMultiplier:         1,
		UsageMultiplierEnabled: true,
		UsageMultiplier:        2,
		Status:                 service.StatusActive,
		SubscriptionType:       service.SubscriptionTypeSubscription,
	}, nil
}

func newGroupUsageMultiplierRouter(adminService service.AdminService, role string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	handler := NewGroupHandler(adminService, nil, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUserRole), role)
		c.Next()
	})
	router.POST("/admin/groups", handler.Create)
	router.PUT("/admin/groups/:id", handler.Update)
	return router
}

func TestGroupCreateMasksUsageMultiplierForOrdinaryAdmin(t *testing.T) {
	stub := &groupUsageMultiplierAdminServiceStub{}
	router := newGroupUsageMultiplierRouter(stub, service.RoleAdmin)
	body := bytes.NewBufferString(`{"name":"hidden","platform":"anthropic","rate_multiplier":1,"usage_multiplier_enabled":true,"usage_multiplier":2}`)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/groups", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, stub.createInput)
	require.False(t, stub.createInput.UsageMultiplierEnabled)
	require.InDelta(t, 1.0, stub.createInput.UsageMultiplier, 1e-12)

	var payload struct {
		Data struct {
			UsageMultiplierEnabled bool    `json:"usage_multiplier_enabled"`
			UsageMultiplier        float64 `json:"usage_multiplier"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.False(t, payload.Data.UsageMultiplierEnabled)
	require.InDelta(t, 1.0, payload.Data.UsageMultiplier, 1e-12)
}

func TestGroupUpdateIgnoresUsageMultiplierForOrdinaryAdmin(t *testing.T) {
	stub := &groupUsageMultiplierAdminServiceStub{}
	router := newGroupUsageMultiplierRouter(stub, service.RoleAdmin)
	body := bytes.NewBufferString(`{"usage_multiplier_enabled":true,"usage_multiplier":2}`)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/admin/groups/11", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, stub.updateInput)
	require.Nil(t, stub.updateInput.UsageMultiplierEnabled)
	require.Nil(t, stub.updateInput.UsageMultiplier)
}

func TestGroupUpdateKeepsUsageMultiplierForSuperAdmin(t *testing.T) {
	stub := &groupUsageMultiplierAdminServiceStub{}
	router := newGroupUsageMultiplierRouter(stub, service.RoleSuperAdmin)
	body := bytes.NewBufferString(`{"usage_multiplier_enabled":true,"usage_multiplier":2}`)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/admin/groups/11", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, stub.updateInput)
	require.NotNil(t, stub.updateInput.UsageMultiplierEnabled)
	require.True(t, *stub.updateInput.UsageMultiplierEnabled)
	require.NotNil(t, stub.updateInput.UsageMultiplier)
	require.InDelta(t, 2.0, *stub.updateInput.UsageMultiplier, 1e-12)
}
