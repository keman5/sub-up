package admin

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAccountUsageStatsForViewHidesAccountCostForAdmin(t *testing.T) {
	stats := &usagestats.AccountUsageStatsResponse{
		History: []usagestats.AccountUsageHistory{{
			Date:       "2026-06-22",
			Cost:       10,
			ActualCost: 8,
			UserCost:   4,
		}},
		Summary: usagestats.AccountUsageSummary{
			TotalCost:         8,
			TotalUserCost:     4,
			TotalStandardCost: 10,
			AvgDailyCost:      8,
			AvgDailyUserCost:  4,
			Today: &struct {
				Date     string  `json:"date"`
				Cost     float64 `json:"cost"`
				UserCost float64 `json:"user_cost"`
				Requests int64   `json:"requests"`
				Tokens   int64   `json:"tokens"`
			}{Date: "2026-06-22", Cost: 8, UserCost: 4},
			HighestCostDay: &struct {
				Date     string  `json:"date"`
				Label    string  `json:"label"`
				Cost     float64 `json:"cost"`
				UserCost float64 `json:"user_cost"`
				Requests int64   `json:"requests"`
			}{Date: "2026-06-22", Cost: 8, UserCost: 4},
			HighestRequestDay: &struct {
				Date     string  `json:"date"`
				Label    string  `json:"label"`
				Requests int64   `json:"requests"`
				Cost     float64 `json:"cost"`
				UserCost float64 `json:"user_cost"`
			}{Date: "2026-06-22", Cost: 8, UserCost: 4},
		},
		Models:            []usagestats.ModelStat{{Model: "gpt-5.5", Cost: 3, ActualCost: 5, AccountCost: 6}},
		Endpoints:         []usagestats.EndpointStat{{Endpoint: "/v1/responses", Cost: 1.2, ActualCost: 2.4}},
		UpstreamEndpoints: []usagestats.EndpointStat{{Endpoint: "https://upstream.example/v1/responses", Cost: 1.4, ActualCost: 2.8}},
	}

	got := accountUsageStatsForView(stats, service.UsageViewPresentation)

	require.NotSame(t, stats, got)
	require.Equal(t, 4.0, got.History[0].Cost)
	require.Equal(t, 4.0, got.History[0].ActualCost)
	require.Equal(t, 4.0, got.Summary.TotalCost)
	require.Equal(t, 4.0, got.Summary.TotalStandardCost)
	require.Equal(t, 4.0, got.Summary.AvgDailyCost)
	require.Equal(t, 4.0, got.Summary.Today.Cost)
	require.Equal(t, 4.0, got.Summary.HighestCostDay.Cost)
	require.Equal(t, 4.0, got.Summary.HighestRequestDay.Cost)
	require.Equal(t, 3.0, got.Models[0].ActualCost)
	require.Zero(t, got.Models[0].AccountCost)
	require.Equal(t, 1.2, got.Endpoints[0].ActualCost)
	require.Equal(t, 1.4, got.UpstreamEndpoints[0].ActualCost)

	require.Equal(t, 8.0, stats.History[0].ActualCost, "presentation view must not mutate raw history")
	require.Equal(t, 8.0, stats.Summary.TotalCost, "presentation view must not mutate raw summary")
	require.Equal(t, 10.0, stats.Summary.TotalStandardCost, "presentation view must not mutate raw summary")
	require.Equal(t, 5.0, stats.Models[0].ActualCost, "presentation view must not mutate raw model stats")
	require.Equal(t, 6.0, stats.Models[0].AccountCost, "presentation view must not mutate raw model stats")
	require.Equal(t, 2.4, stats.Endpoints[0].ActualCost, "presentation view must not mutate raw endpoint stats")
	require.Equal(t, 2.8, stats.UpstreamEndpoints[0].ActualCost, "presentation view must not mutate raw upstream endpoint stats")
}

func TestAccountUsageStatsForViewKeepsRawForSuperAdmin(t *testing.T) {
	stats := &usagestats.AccountUsageStatsResponse{
		History: []usagestats.AccountUsageHistory{{ActualCost: 8, UserCost: 4}},
		Summary: usagestats.AccountUsageSummary{
			TotalCost:     8,
			TotalUserCost: 4,
		},
		Models:            []usagestats.ModelStat{{Cost: 3, ActualCost: 5, AccountCost: 6}},
		Endpoints:         []usagestats.EndpointStat{{Cost: 1.2, ActualCost: 2.4}},
		UpstreamEndpoints: []usagestats.EndpointStat{{Cost: 1.4, ActualCost: 2.8}},
	}

	got := accountUsageStatsForView(stats, service.UsageViewRaw)

	require.Same(t, stats, got)
	require.Equal(t, 8.0, got.History[0].ActualCost)
	require.Equal(t, 8.0, got.Summary.TotalCost)
	require.Equal(t, 5.0, got.Models[0].ActualCost)
	require.Equal(t, 6.0, got.Models[0].AccountCost)
	require.Equal(t, 2.4, got.Endpoints[0].ActualCost)
	require.Equal(t, 2.8, got.UpstreamEndpoints[0].ActualCost)
}
