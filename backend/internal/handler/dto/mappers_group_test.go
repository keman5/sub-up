//go:build unit

package dto

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGroupFromServiceMasksUsagePresentationMultiplierForUsers(t *testing.T) {
	t.Parallel()

	group := &service.Group{
		ID:                     1,
		Name:                   "subscription",
		Platform:               service.PlatformAnthropic,
		RateMultiplier:         1,
		DisplayRateMultiplier:  1,
		UsageMultiplierEnabled: true,
		UsageMultiplier:        2,
		Status:                 service.StatusActive,
		SubscriptionType:       service.SubscriptionTypeSubscription,
	}

	userDTO := GroupFromService(group)
	require.NotNil(t, userDTO)
	require.False(t, userDTO.UsageMultiplierEnabled)
	require.InDelta(t, 1.0, userDTO.UsageMultiplier, 1e-12)

	adminDTO := GroupFromServiceAdminWithViewer(group, service.UsageViewRaw)
	require.NotNil(t, adminDTO)
	require.True(t, adminDTO.UsageMultiplierEnabled)
	require.InDelta(t, 2.0, adminDTO.UsageMultiplier, 1e-12)

	ordinaryAdminDTO := GroupFromServiceAdminWithViewer(group, service.UsageViewPresentation)
	require.NotNil(t, ordinaryAdminDTO)
	require.False(t, ordinaryAdminDTO.UsageMultiplierEnabled)
	require.InDelta(t, 1.0, ordinaryAdminDTO.UsageMultiplier, 1e-12)
}
