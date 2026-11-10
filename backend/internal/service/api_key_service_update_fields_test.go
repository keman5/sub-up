//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// api_keys 的 quota_used / usage_5h|1d|7d 由计费热路径原子递增。
// 编辑 Key（改名、换分组……）若整行回写，并发累计的用量就会被旧快照覆盖。
// 这些用例锁死"只声明请求真正要改的列"。

type updateFieldsAPIKeyRepoStub struct {
	quotaBaseAPIKeyRepoStub
	key          *APIKey
	updateFields []APIKeyUpdateFields
	updatedKey   *APIKey
}

// IncrementQuotaUsed 模拟计费热路径上的原子递增：只动 quota_used。
func (s *updateFieldsAPIKeyRepoStub) IncrementQuotaUsed(_ context.Context, _ int64, amount float64) (float64, error) {
	s.key.QuotaUsed += amount
	return s.key.QuotaUsed, nil
}

func (s *updateFieldsAPIKeyRepoStub) GetByID(context.Context, int64) (*APIKey, error) {
	clone := *s.key
	return &clone, nil
}

func (s *updateFieldsAPIKeyRepoStub) Update(_ context.Context, key *APIKey, fields APIKeyUpdateFields) error {
	s.updateFields = append(s.updateFields, fields)
	clone := *key
	s.updatedKey = &clone
	return nil
}

type resetWindowCacheSpy struct{ ids []int64 }

func (s *resetWindowCacheSpy) InvalidateAPIKeyRateLimit(_ context.Context, id int64) error {
	s.ids = append(s.ids, id)
	return nil
}

func TestAPIKeyUpdate_SingleWindowResetPersistsOnlyRequestedWindow(t *testing.T) {
	for _, window := range []string{"5h", "1d", "7d"} {
		t.Run(window, func(t *testing.T) {
			start := time.Now().Add(-time.Hour).Truncate(time.Second)
			key := &APIKey{ID: 1, UserID: 7, Key: "sk-window", Status: StatusActive,
				Quota: 100, QuotaUsed: 19, RateLimit5h: 50, RateLimit1d: 100, RateLimit7d: 200,
				Usage5h: 11, Usage1d: 22, Usage7d: 33,
				Window5hStart: &start, Window1dStart: &start, Window7dStart: &start}
			svc, repo := newUpdateFieldsAPIKeyService(key)
			cache := &resetWindowCacheSpy{}
			svc.SetRateLimitCacheInvalidator(cache)
			got, err := svc.Update(context.Background(), 1, 7, UpdateAPIKeyRequest{ResetRateLimitWindow: &window})
			require.NoError(t, err)
			require.Len(t, repo.updateFields, 1)
			require.False(t, repo.updateFields[0].IsEmpty(), "reset must declare persistence columns")
			want := *key
			wantFields := APIKeyUpdateFields{}
			switch window {
			case "5h":
				want.Usage5h, want.Window5hStart = 0, nil
				wantFields.RateLimitUsage5h = true
			case "1d":
				want.Usage1d, want.Window1dStart = 0, nil
				wantFields.RateLimitUsage1d = true
			case "7d":
				want.Usage7d, want.Window7dStart = 0, nil
				wantFields.RateLimitUsage7d = true
			}
			require.Equal(t, wantFields, repo.updateFields[0])
			require.Equal(t, want, *repo.updatedKey, "actual repository update payload")
			require.NotNil(t, got.CompiledIPWhitelist)
			require.NotNil(t, got.CompiledIPBlacklist)
			returned := *got
			returned.CompiledIPWhitelist, returned.CompiledIPBlacklist = nil, nil
			require.Equal(t, want, returned)
			require.Equal(t, []int64{1}, cache.ids)
		})
	}
}

func newUpdateFieldsAPIKeyService(key *APIKey) (*APIKeyService, *updateFieldsAPIKeyRepoStub) {
	repo := &updateFieldsAPIKeyRepoStub{key: key}
	return &APIKeyService{apiKeyRepo: repo}, repo
}

func TestAPIKeyUpdate_OnlyDeclaresRequestedColumns(t *testing.T) {
	name := "renamed"
	quota := 500.0
	rateLimit := 42.0
	whitelist := []string{"10.0.0.1"}

	tests := []struct {
		name string
		req  UpdateAPIKeyRequest
		want APIKeyUpdateFields
	}{
		{
			name: "name only",
			req:  UpdateAPIKeyRequest{Name: &name},
			want: APIKeyUpdateFields{Name: true},
		},
		{
			name: "quota only",
			req:  UpdateAPIKeyRequest{Quota: &quota},
			want: APIKeyUpdateFields{Quota: true},
		},
		{
			name: "rate limit threshold only",
			req:  UpdateAPIKeyRequest{RateLimit5h: &rateLimit},
			want: APIKeyUpdateFields{RateLimits: true},
		},
		{
			name: "ip whitelist only",
			req:  UpdateAPIKeyRequest{IPWhitelist: &whitelist},
			want: APIKeyUpdateFields{IPRules: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo := newUpdateFieldsAPIKeyService(&APIKey{
				ID:        1,
				UserID:    7,
				Key:       "sk-test",
				Name:      "before",
				Status:    StatusActive,
				Quota:     100,
				QuotaUsed: 30,
				Usage5h:   12,
			})

			_, err := svc.Update(context.Background(), 1, 7, tt.req)
			require.NoError(t, err)
			require.Equal(t, []APIKeyUpdateFields{tt.want}, repo.updateFields)
		})
	}
}

// 显式重置仍需声明对应的列，避免收窄写入列时把功能改坏。
func TestAPIKeyUpdate_DeclaresUsageColumnsOnExplicitReset(t *testing.T) {
	reset := true
	svc, repo := newUpdateFieldsAPIKeyService(&APIKey{
		ID: 1, UserID: 7, Key: "sk-test", Status: StatusActive, Quota: 100, QuotaUsed: 30, Usage5h: 12,
	})

	_, err := svc.Update(context.Background(), 1, 7, UpdateAPIKeyRequest{
		ResetQuota:          &reset,
		ResetRateLimitUsage: &reset,
	})
	require.NoError(t, err)
	require.Equal(t, []APIKeyUpdateFields{{QuotaUsed: true, RateLimitUsage: true}}, repo.updateFields)
}

// 配额扩容会顺带把 quota_exhausted 复活为 active，此时必须声明 status。
func TestAPIKeyUpdate_DeclaresStatusWhenReactivated(t *testing.T) {
	quota := 500.0
	svc, repo := newUpdateFieldsAPIKeyService(&APIKey{
		ID: 1, UserID: 7, Key: "sk-test", Status: StatusAPIKeyQuotaExhausted, Quota: 100, QuotaUsed: 100,
	})

	_, err := svc.Update(context.Background(), 1, 7, UpdateAPIKeyRequest{Quota: &quota})
	require.NoError(t, err)
	require.Equal(t, []APIKeyUpdateFields{{Quota: true, Status: true}}, repo.updateFields)
}

// 计费热路径把 Key 标记为配额耗尽时只写 status，
// 否则会把刚原子递增的 quota_used 按快照覆盖掉。
func TestUpdateQuotaUsed_ExhaustedMarkOnlyDeclaresStatus(t *testing.T) {
	repo := &updateFieldsAPIKeyRepoStub{key: &APIKey{
		ID: 1, UserID: 7, Key: "sk-test", Status: StatusActive, Quota: 10, QuotaUsed: 10,
	}}
	svc := &APIKeyService{apiKeyRepo: repo}

	require.NoError(t, svc.UpdateQuotaUsed(context.Background(), 1, 5))
	require.Equal(t, []APIKeyUpdateFields{{Status: true}}, repo.updateFields)
}
