package dto

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"reflect"
	"testing"
)

func TestAccountUsageInfoForViewerProjectsEveryLocalStatsFieldWithoutMutatingCache(t *testing.T) {
	cost := 123.0
	source := &service.UsageInfo{GrokBilling: &xai.BillingSummary{MonthlyUsed: &cost, MonthlyLimit: &cost}}
	// Exercise every local stats-bearing field, including Codex and new provider windows.
	value := reflect.ValueOf(source).Elem()
	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		if field.Type() == reflect.TypeOf((*service.UsageProgress)(nil)) {
			field.Set(reflect.ValueOf(&service.UsageProgress{Utilization: 53, UsedRequests: 8, LimitRequests: 20, WindowStats: &service.WindowStats{Cost: 123, StandardCost: 456, UserCost: 7, Tokens: 1000}}))
		}
		if field.Type() == reflect.TypeOf((*service.WindowStats)(nil)) {
			field.Set(reflect.ValueOf(&service.WindowStats{Cost: 123, StandardCost: 456, UserCost: 7, Tokens: 1000}))
		}
	}
	got := AccountUsageInfoForViewer(source, service.UsageViewPresentation)
	require.NotSame(t, source, got)
	projected := reflect.ValueOf(got).Elem()
	for i := 0; i < projected.NumField(); i++ {
		field := projected.Field(i)
		if w, ok := field.Interface().(*service.UsageProgress); ok && w != nil {
			require.Equal(t, 53.0, w.Utilization)
			require.EqualValues(t, 8, w.UsedRequests)
			require.EqualValues(t, 20, w.LimitRequests)
			require.Equal(t, 7.0, w.WindowStats.Cost)
			require.Equal(t, 7.0, w.WindowStats.StandardCost)
			require.EqualValues(t, 1000, w.WindowStats.Tokens)
		}
		if s, ok := field.Interface().(*service.WindowStats); ok && s != nil {
			require.Equal(t, 7.0, s.Cost)
			require.Equal(t, 7.0, s.StandardCost)
		}
	}
	require.Equal(t, 123.0, source.FiveHour.WindowStats.Cost)
	require.Equal(t, 123.0, *got.GrokBilling.MonthlyUsed, "upstream billing quota is not a local usage cost")
	require.Same(t, source, AccountUsageInfoForViewer(source, service.UsageViewRaw))
	require.Nil(t, AccountUsageInfoForViewer(nil, service.UsageViewPresentation))
	require.Nil(t, AccountUsageInfoForViewer(&service.UsageInfo{}, service.UsageViewPresentation).FiveHour)
}
