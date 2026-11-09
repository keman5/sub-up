//go:build unit

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// Exercise the real GetByKey cold/load and warm/L2 paths through JSON.
type f04JSONAuthCache struct {
	authCacheStub
	payload []byte
}

func (c *f04JSONAuthCache) GetAuthCache(context.Context, string) (*APIKeyAuthCacheEntry, error) {
	if c.payload == nil {
		return nil, nil
	}
	var entry APIKeyAuthCacheEntry
	err := json.Unmarshal(c.payload, &entry)
	return &entry, err
}

func (c *f04JSONAuthCache) SetAuthCache(_ context.Context, _ string, entry *APIKeyAuthCacheEntry, _ time.Duration) error {
	var err error
	c.payload, err = json.Marshal(entry)
	return err
}

func TestF04AuthUsageMultiplierColdWarmAndV27Refresh(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, enabled := range []bool{false, true} {
			t.Run(fmt.Sprintf("v27=%v/enabled=%v", legacy, enabled), func(t *testing.T) {
				ctx := context.Background()
				key := profitAuthTestAPIKey()
				key.Group.UsageMultiplierEnabled = enabled
				key.Group.UsageMultiplier = 3.5
				cache := &f04JSONAuthCache{}
				if legacy {
					// Actual pre-fix wire shape: v27 has no usage multiplier fields.
					cache.payload = []byte(`{"snapshot":{"version":27,"group":{"id":50}}}`)
					old, err := cache.GetAuthCache(ctx, "unused")
					require.NoError(t, err)
					got, used, err := (&APIKeyService{}).applyAuthCacheEntry(key.Key, old)
					require.NoError(t, err)
					require.False(t, used)
					require.Nil(t, got)
				}
				calls := 0
				repo := &authRepoStub{getByKeyForAuth: func(context.Context, string) (*APIKey, error) {
					calls++
					return key, nil
				}}
				svc := NewAPIKeyService(repo, nil, nil, nil, nil, cache, &config.Config{APIKeyAuth: config.APIKeyAuthCacheConfig{L2TTLSeconds: 60}})
				for _, path := range []string{"cold-or-stale", "warm"} {
					got, err := svc.GetByKey(ctx, "sk-f04")
					require.NoError(t, err, path)
					require.Equal(t, enabled, got.Group.UsageMultiplierEnabled, path)
					require.Equal(t, 3.5, got.Group.UsageMultiplier, path)
					require.Equal(t, key.Group.RateMultiplier, got.Group.RateMultiplier)
					wantPresentation := 1.0
					if enabled {
						wantPresentation = 3.5
					}
					require.Equal(t, wantPresentation, ResolvePresentationMultiplier(got.Group, 1000, 0, 0, 0))
				}
				require.Equal(t, 1, calls, "warm hit must not consult the repository")
				stored, err := cache.GetAuthCache(ctx, "unused")
				require.NoError(t, err)
				require.Equal(t, apiKeyAuthSnapshotVersion, stored.Snapshot.Version)
				require.Greater(t, stored.Snapshot.Version, 27)
			})
		}
	}
}

func TestF04DuplicatePreservesSevenLocalFieldsWithoutAliasing(t *testing.T) {
	total, fallback := 99.0, int64(13)
	source := &Group{Name: "source", RateMultiplier: 0.25, DisplayRateMultiplier: 1.7, UsageMultiplierEnabled: true, UsageMultiplier: 2.5, TotalLimitUSD: &total, QuotaFallbackGroupID: &fallback, ModelPolicyMode: GroupModelPolicyModeForce, ModelPolicyModel: "gpt-5.3-codex-spark"}
	copy := cloneGroupForDuplicate(source, "op")
	require.Equal(t, source.DisplayRateMultiplier, copy.DisplayRateMultiplier)
	require.Equal(t, source.UsageMultiplierEnabled, copy.UsageMultiplierEnabled)
	require.Equal(t, source.UsageMultiplier, copy.UsageMultiplier)
	require.Equal(t, source.ModelPolicyMode, copy.ModelPolicyMode)
	require.Equal(t, source.ModelPolicyModel, copy.ModelPolicyModel)
	require.Equal(t, *source.TotalLimitUSD, *copy.TotalLimitUSD)
	require.Equal(t, *source.QuotaFallbackGroupID, *copy.QuotaFallbackGroupID)
	require.NotSame(t, source.TotalLimitUSD, copy.TotalLimitUSD)
	require.NotSame(t, source.QuotaFallbackGroupID, copy.QuotaFallbackGroupID)
	*copy.TotalLimitUSD, *copy.QuotaFallbackGroupID = 3, 4
	copy.DisplayRateMultiplier, copy.UsageMultiplier = 8, 9
	copy.UsageMultiplierEnabled = false
	copy.ModelPolicyMode, copy.ModelPolicyModel = "", ""
	require.Equal(t, 99.0, *source.TotalLimitUSD)
	require.EqualValues(t, 13, *source.QuotaFallbackGroupID)
	require.Equal(t, 1.7, source.DisplayRateMultiplier)
	require.Equal(t, 2.5, source.UsageMultiplier)
	require.True(t, source.UsageMultiplierEnabled)
	require.Equal(t, GroupModelPolicyModeForce, source.ModelPolicyMode)
	require.Equal(t, "gpt-5.3-codex-spark", source.ModelPolicyModel)
	require.Equal(t, 0.25, copy.RateMultiplier)
	empty := cloneGroupForDuplicate(&Group{}, "op")
	require.Nil(t, empty.TotalLimitUSD)
	require.Nil(t, empty.QuotaFallbackGroupID)
}
