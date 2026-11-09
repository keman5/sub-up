package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestF04GroupHandlerForwardsDisplayRateAndOptionalLimits(t *testing.T) {
	for _, role := range []string{service.RoleAdmin, service.RoleSuperAdmin} {
		for _, method := range []string{http.MethodPost, http.MethodPut} {
			for _, explicit := range []bool{false, true} {
				t.Run(role+"/"+method+"/explicit="+map[bool]string{false: "false", true: "true"}[explicit], func(t *testing.T) {
					stub := &stubAdminService{}
					router := newGroupUsageMultiplierRouter(stub, role)
					body := `{"name":"display","rate_multiplier":0.25}`
					if explicit {
						body = `{"name":"display","rate_multiplier":0.25,"display_rate_multiplier":1.75,"daily_limit_usd":null,"weekly_limit_usd":0,"total_limit_usd":null}`
					}
					path := "/admin/groups"
					if method == http.MethodPut {
						path += "/11"
					}
					w := httptest.NewRecorder()
					req := httptest.NewRequest(method, path, strings.NewReader(body))
					req.Header.Set("Content-Type", "application/json")
					router.ServeHTTP(w, req)
					require.Equal(t, http.StatusOK, w.Code, w.Body.String())
					var display, daily, weekly, monthly, total *float64
					if method == http.MethodPost {
						require.Len(t, stub.createdGroups, 1)
						in := stub.createdGroups[0]
						display, daily, weekly, monthly, total = in.DisplayRateMultiplier, in.DailyLimitUSD, in.WeeklyLimitUSD, in.MonthlyLimitUSD, in.TotalLimitUSD
						require.Equal(t, 0.25, in.RateMultiplier)
					} else {
						require.Len(t, stub.updatedGroups, 1)
						in := stub.updatedGroups[0]
						display, daily, weekly, monthly, total = in.DisplayRateMultiplier, in.DailyLimitUSD, in.WeeklyLimitUSD, in.MonthlyLimitUSD, in.TotalLimitUSD
						require.Equal(t, 0.25, *in.RateMultiplier)
					}
					require.Nil(t, monthly)
					if explicit {
						require.NotNil(t, display)
						require.Equal(t, 1.75, *display)
						require.NotNil(t, daily)
						require.Negative(t, *daily)
						require.NotNil(t, total)
						require.Negative(t, *total)
						require.NotNil(t, weekly)
						require.Zero(t, *weekly)
					} else {
						require.Nil(t, display)
						require.Nil(t, daily)
						require.Nil(t, weekly)
						require.Nil(t, total)
					}
				})
			}
		}
	}
}
