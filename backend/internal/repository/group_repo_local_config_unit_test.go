package repository

import (
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"
)

func TestGroupEntityToServiceUsageMultiplier(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		entity := &dbent.Group{UsageMultiplierEnabled: enabled, UsageMultiplier: 2.5}
		got := groupEntityToService(entity)
		require.Equal(t, enabled, got.UsageMultiplierEnabled)
		require.Equal(t, 2.5, got.UsageMultiplier)
		require.Equal(t, 2.5, entity.UsageMultiplier)
	}
	require.Nil(t, groupEntityToService(nil))
}
