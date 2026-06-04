package dto

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGroupFromService_UsesDisplayRateMultiplierForUserDTO(t *testing.T) {
	t.Parallel()

	group := &service.Group{
		ID:                    1,
		Name:                  "premium",
		Platform:              service.PlatformAnthropic,
		RateMultiplier:        2,
		DisplayRateMultiplier: 1,
	}

	userDTO := GroupFromService(group)
	adminDTO := GroupFromServiceAdmin(group)

	require.Equal(t, 1.0, userDTO.RateMultiplier)
	require.Equal(t, 2.0, adminDTO.RateMultiplier)
	require.Equal(t, 2.0, adminDTO.BillingRateMultiplier)
	require.Equal(t, 1.0, adminDTO.DisplayRateMultiplier)
}
