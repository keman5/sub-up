package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type groupUsageSummaryRepoCapture struct {
	service.UsageLogRepository
	usePresentation bool
}

func (r *groupUsageSummaryRepoCapture) GetAllGroupUsageSummaryForView(ctx context.Context, todayStart time.Time, usePresentation bool) ([]usagestats.GroupUsageSummary, error) {
	r.usePresentation = usePresentation
	return []usagestats.GroupUsageSummary{{GroupID: 1, TodayCost: 2, TotalCost: 3}}, nil
}

func newGroupUsageSummaryTestRouter(repo *groupUsageSummaryRepoCapture, role string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	dashboardSvc := service.NewDashboardService(repo, nil, nil, nil)
	handler := NewGroupHandler(nil, dashboardSvc, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUserRole), role)
		c.Next()
	})
	router.GET("/admin/groups/usage-summary", handler.GetUsageSummary)
	return router
}

func TestGroupUsageSummaryUsesPresentationForOrdinaryAdmin(t *testing.T) {
	repo := &groupUsageSummaryRepoCapture{}
	router := newGroupUsageSummaryTestRouter(repo, service.RoleAdmin)

	req := httptest.NewRequest(http.MethodGet, "/admin/groups/usage-summary", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, repo.usePresentation)
}

func TestGroupUsageSummaryUsesRawForSuperAdmin(t *testing.T) {
	repo := &groupUsageSummaryRepoCapture{}
	router := newGroupUsageSummaryTestRouter(repo, service.RoleSuperAdmin)

	req := httptest.NewRequest(http.MethodGet, "/admin/groups/usage-summary", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.False(t, repo.usePresentation)
}
