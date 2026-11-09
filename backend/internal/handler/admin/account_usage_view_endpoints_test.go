//go:build unit

package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
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

type accountViewerStatsRepo struct{ service.UsageLogRepository }

func (*accountViewerStatsRepo) GetAccountTodayStats(context.Context, int64) (*usagestats.AccountStats, error) {
	return &usagestats.AccountStats{Cost: 123, StandardCost: 456, UserCost: 7, Tokens: 1000}, nil
}
func (*accountViewerStatsRepo) GetAccountWindowStats(context.Context, int64, time.Time) (*usagestats.AccountStats, error) {
	return &usagestats.AccountStats{Cost: 123, StandardCost: 456, UserCost: 7, Tokens: 1000}, nil
}
func (*accountViewerStatsRepo) GetAccountWindowStatsBatch(context.Context, []int64, time.Time) (map[int64]*usagestats.AccountStats, error) {
	return map[int64]*usagestats.AccountStats{1: {Cost: 123, StandardCost: 456, UserCost: 7, Tokens: 1000}}, nil
}

type accountViewerAccountRepo struct {
	service.AccountRepository
	account *service.Account
}

func (r *accountViewerAccountRepo) GetByID(context.Context, int64) (*service.Account, error) {
	return r.account, nil
}
func (r *accountViewerAccountRepo) GetByIDs(context.Context, []int64) ([]*service.Account, error) {
	return []*service.Account{r.account}, nil
}

func TestAccountUsageEndpointsRespectViewerAndScopedPermissions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	end := time.Now().Add(time.Hour)
	a := &service.Account{ID: 1, Platform: service.PlatformAnthropic, Type: service.AccountTypeSetupToken, GroupIDs: []int64{1}, SessionWindowEnd: &end, Extra: map[string]any{"session_window_utilization": 0.53}}
	usage := service.NewAccountUsageService(&accountViewerAccountRepo{account: a}, &accountViewerStatsRepo{}, nil, nil, nil, nil, nil, nil, service.NewUsageCache(), nil, nil)
	h := &AccountHandler{adminService: &scopedStatisticsAccounts{accounts: map[int64]*service.Account{1: a}}, accountUsageService: usage}
	for _, role := range []string{service.RoleAdmin, service.RoleSuperAdmin} {
		for _, scoped := range []bool{false, true} {
			for _, ep := range []struct {
				method, path, route, costPath string
				call                          gin.HandlerFunc
			}{
				{"GET", "/1/today-stats", "/:id/today-stats", "data.cost", h.GetTodayStats},
				{"POST", "/today-stats/batch", "/today-stats/batch", "data.stats.1.cost", h.GetBatchTodayStats},
				{"GET", "/1/usage?source=passive", "/:id/usage", "data.five_hour.window_stats.cost", h.GetUsage},
				{"GET", "/1/usage?source=active&force=true", "/:id/usage", "data.five_hour.window_stats.cost", h.GetUsage},
				{"POST", "/usage/batch", "/usage/batch", "data.usage.1.five_hour.window_stats.cost", h.GetBatchUsage},
			} {
				t.Run(fmt.Sprintf("%s/scoped=%v/%s%s", role, scoped, ep.method, ep.path), func(t *testing.T) {
					r := gin.New()
					r.Use(func(c *gin.Context) {
						c.Set("user_role", role)
						if scoped {
							c.Request = c.Request.WithContext(adminaccess.WithPrincipal(c.Request.Context(), &adminaccess.Principal{Policy: adminaccess.Policy{Rules: []adminaccess.Rule{{Permissions: []string{"accounts.usage.read", "accounts.stats.read"}, GroupIDs: []int64{1}, Platforms: []string{service.PlatformAnthropic}, AccountTypes: []string{service.AccountTypeSetupToken}}}}}))
						}
					})
					if scoped {
						r.Use(h.ScopedAdminKeyAccounts)
					}
					r.Handle(ep.method, "/api/v1/admin/accounts"+ep.route, ep.call)
					w := httptest.NewRecorder()
					req := httptest.NewRequest(ep.method, "/api/v1/admin/accounts"+ep.path, strings.NewReader(`{"account_ids":[1]}`))
					req.Header.Set("Content-Type", "application/json")
					r.ServeHTTP(w, req)
					require.Equal(t, http.StatusOK, w.Code, w.Body.String())
					want := 7.0
					if role == service.RoleSuperAdmin {
						want = 123
					}
					require.Equal(t, want, gjson.GetBytes(w.Body.Bytes(), ep.costPath).Float(), w.Body.String())
					standardCost := 7.0
					if role == service.RoleSuperAdmin {
						standardCost = 456
					}
					require.Equal(t, standardCost, gjson.GetBytes(w.Body.Bytes(), strings.TrimSuffix(ep.costPath, "cost")+"standard_cost").Float(), w.Body.String())
					if strings.Contains(ep.path, "usage") {
						prefix := strings.TrimSuffix(ep.costPath, "window_stats.cost")
						require.Equal(t, 53.0, gjson.GetBytes(w.Body.Bytes(), prefix+"utilization").Float())
					}
				})
			}
		}
	}
	// The shared service window cache must still contain raw costs after admin reads.
	raw, err := usage.GetPassiveUsage(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, 123.0, raw.FiveHour.WindowStats.Cost)
}
func TestScopedUsageViewerDoesNotGrantStatsPermission(t *testing.T) {
	a := &service.Account{ID: 1, Platform: "openai", Type: "oauth", GroupIDs: []int64{1}}
	u := &service.UsageInfo{FiveHour: &service.UsageProgress{Utilization: 53, WindowStats: &service.WindowStats{Cost: 123, StandardCost: 456, UserCost: 7}}, GrokLocalUsage: &service.WindowStats{Cost: 123, UserCost: 7}}
	for _, mode := range []service.UsageViewMode{service.UsageViewPresentation, service.UsageViewRaw} {
		out := scopedUsageDTO(u, statisticsPolicy("accounts.usage.read"), a, mode)
		b, err := json.Marshal(out)
		require.NoError(t, err)
		require.NotContains(t, string(b), "window_stats")
		require.NotContains(t, string(b), "grok_local_usage")
	}
	require.Equal(t, 123.0, u.FiveHour.WindowStats.Cost)
}
