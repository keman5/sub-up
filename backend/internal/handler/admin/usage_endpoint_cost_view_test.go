package admin

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUsageStatsEndpointCostsRespectViewerWithoutMutatingCache(t *testing.T) {
	raw := &usagestats.UsageStats{
		Endpoints:         []usagestats.EndpointStat{{AccountCost: 12}},
		UpstreamEndpoints: []usagestats.EndpointStat{{AccountCost: 13}},
		EndpointPaths:     []usagestats.EndpointStat{{AccountCost: 14}},
	}
	view := usageStatsForView(raw, service.UsageViewPresentation)
	require.Zero(t, view.Endpoints[0].AccountCost)
	require.Zero(t, view.UpstreamEndpoints[0].AccountCost)
	require.Zero(t, view.EndpointPaths[0].AccountCost)
	require.Equal(t, 12.0, raw.Endpoints[0].AccountCost)
	require.Equal(t, 13.0, raw.UpstreamEndpoints[0].AccountCost)
	require.Equal(t, 14.0, raw.EndpointPaths[0].AccountCost)
	require.Same(t, raw, usageStatsForView(raw, service.UsageViewRaw))
}
