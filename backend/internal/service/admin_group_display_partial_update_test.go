//go:build unit

package service

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestF04AdminGroupDisplayRateCreateUpdateAndValidation(t *testing.T) {
	ctx := context.Background()
	for _, value := range []*float64{nil, groupDuplicateTestPointer(1.75)} {
		repo := &groupRepoStubForAdmin{}
		svc := &adminServiceImpl{groupRepo: repo}
		got, err := svc.CreateGroup(ctx, &CreateGroupInput{Name: "display", Platform: PlatformAnthropic, RateMultiplier: 0.25, DisplayRateMultiplier: value})
		require.NoError(t, err)
		want := 1.0
		if value != nil {
			want = *value
		}
		require.Equal(t, want, got.DisplayRateMultiplier)
		require.Equal(t, want, repo.created.DisplayRateMultiplier)
		require.Equal(t, 0.25, repo.created.RateMultiplier)
	}
	for _, value := range []*float64{nil, groupDuplicateTestPointer(2.5)} {
		repo := &groupPlatformRepoStub{group: &Group{ID: 7, Name: "display", Platform: PlatformAnthropic, RateMultiplier: 0.25, DisplayRateMultiplier: 1.75, UsageMultiplierEnabled: true, UsageMultiplier: 3}}
		svc := &adminServiceImpl{groupRepo: repo}
		got, err := svc.UpdateGroup(ctx, 7, &UpdateGroupInput{DisplayRateMultiplier: value})
		require.NoError(t, err)
		want := 1.75
		if value != nil {
			want = *value
		}
		require.Equal(t, want, got.DisplayRateMultiplier)
		require.Equal(t, 0.25, repo.updated.RateMultiplier)
		require.True(t, repo.updated.UsageMultiplierEnabled)
		require.Equal(t, 3.0, repo.updated.UsageMultiplier)
	}
	for _, value := range []float64{0, -1, math.NaN(), math.Inf(1), math.Inf(-1)} {
		repo := &groupPlatformRepoStub{group: &Group{ID: 7, Platform: PlatformAnthropic, DisplayRateMultiplier: 1.75}}
		svc := &adminServiceImpl{groupRepo: repo}
		_, err := svc.CreateGroup(ctx, &CreateGroupInput{Name: "invalid", RateMultiplier: 1, DisplayRateMultiplier: &value})
		require.ErrorContains(t, err, "display_rate_multiplier")
		_, err = svc.UpdateGroup(ctx, 7, &UpdateGroupInput{DisplayRateMultiplier: &value})
		require.ErrorContains(t, err, "display_rate_multiplier")
		require.Nil(t, repo.updated)
	}
}

func TestF04AdminGroupPartialUpdateLimits(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value *float64
		want  *float64
	}{
		{"omitted", nil, groupDuplicateTestPointer(42.0)},
		{"unlimited", groupDuplicateTestPointer(-1.0), nil},
		{"zero", groupDuplicateTestPointer(0.0), groupDuplicateTestPointer(0.0)},
		{"positive", groupDuplicateTestPointer(6.5), groupDuplicateTestPointer(6.5)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			limit := 42.0
			repo := &groupPlatformRepoStub{group: &Group{ID: 7, Platform: PlatformAnthropic, DailyLimitUSD: &limit, WeeklyLimitUSD: &limit, MonthlyLimitUSD: &limit, TotalLimitUSD: &limit}}
			got, err := (&adminServiceImpl{groupRepo: repo}).UpdateGroup(context.Background(), 7, &UpdateGroupInput{Name: "partial", DailyLimitUSD: tc.value, WeeklyLimitUSD: tc.value, MonthlyLimitUSD: tc.value, TotalLimitUSD: tc.value})
			require.NoError(t, err)
			for _, actual := range []*float64{got.DailyLimitUSD, got.WeeklyLimitUSD, got.MonthlyLimitUSD, got.TotalLimitUSD} {
				require.Equal(t, tc.want, actual)
			}
		})
	}
}
