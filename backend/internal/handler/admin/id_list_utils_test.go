//go:build unit

package admin

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestNormalizeInt64IDList(t *testing.T) {
	tests := []struct {
		name string
		in   []int64
		want []int64
	}{
		{"nil input", nil, nil},
		{"empty input", []int64{}, nil},
		{"single element", []int64{5}, []int64{5}},
		{"already sorted unique", []int64{1, 2, 3}, []int64{1, 2, 3}},
		{"duplicates removed", []int64{3, 1, 3, 2, 1}, []int64{1, 2, 3}},
		{"zero filtered", []int64{0, 1, 2}, []int64{1, 2}},
		{"negative filtered", []int64{-5, -1, 3}, []int64{3}},
		{"all invalid", []int64{0, -1, -2}, []int64{}},
		{"sorted output", []int64{9, 3, 7, 1}, []int64{1, 3, 7, 9}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeInt64IDList(tc.in)
			if tc.want == nil {
				require.Nil(t, got)
			} else {
				require.Equal(t, tc.want, got)
			}
		})
	}
}

func TestBuildAccountTodayStatsBatchCacheKey(t *testing.T) {
	tests := []struct {
		name string
		ids  []int64
		mode service.UsageViewMode
		want string
	}{
		{"empty presentation", nil, service.UsageViewPresentation, "accounts_today_stats:presentation:empty"},
		{"single presentation", []int64{42}, service.UsageViewPresentation, "accounts_today_stats:presentation:42"},
		{"multiple raw", []int64{1, 2, 3}, service.UsageViewRaw, "accounts_today_stats:raw:1,2,3"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := buildAccountTodayStatsBatchCacheKey(tc.ids, tc.mode)
			require.Equal(t, tc.want, got)
		})
	}
}
