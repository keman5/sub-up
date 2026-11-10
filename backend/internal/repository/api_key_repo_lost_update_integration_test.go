//go:build integration

package repository

import (
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// A single-window reset must not write the other windows from a stale snapshot.
func (s *APIKeyRepoSuite) TestUpdate_SingleWindowResetPreservesConcurrentUsage() {
	for _, window := range []string{"5h", "1d", "7d"} {
		s.Run(window, func() {
			user := s.mustCreateUser(fmt.Sprintf("apikey-reset-%s@example.com", window))
			key := &service.APIKey{UserID: user.ID, Key: "sk-single-reset-" + window,
				Name: "preserved", Status: service.StatusActive, Quota: 100,
				RateLimit5h: 100, RateLimit1d: 200, RateLimit7d: 300}
			s.Require().NoError(s.repo.Create(s.ctx, key))
			s.Require().NoError(s.repo.IncrementRateLimitUsage(s.ctx, key.ID, 12))
			stale, err := s.repo.GetByID(s.ctx, key.ID)
			s.Require().NoError(err)
			s.Require().NoError(s.repo.IncrementRateLimitUsage(s.ctx, key.ID, 30))
			_, err = s.repo.IncrementQuotaUsed(s.ctx, key.ID, 9)
			s.Require().NoError(err)
			before, err := s.repo.GetByID(s.ctx, key.ID)
			s.Require().NoError(err)
			fields := service.APIKeyUpdateFields{}
			switch window {
			case "5h":
				stale.Usage5h, stale.Window5hStart = 0, nil
				fields.RateLimitUsage5h = true
			case "1d":
				stale.Usage1d, stale.Window1dStart = 0, nil
				fields.RateLimitUsage1d = true
			case "7d":
				stale.Usage7d, stale.Window7dStart = 0, nil
				fields.RateLimitUsage7d = true
			}
			s.Require().NoError(s.repo.Update(s.ctx, stale, fields))
			got, err := s.repo.GetByID(s.ctx, key.ID)
			s.Require().NoError(err)
			for _, other := range []struct {
				name               string
				usage              float64
				start, beforeStart *time.Time
			}{
				{"5h", got.Usage5h, got.Window5hStart, before.Window5hStart},
				{"1d", got.Usage1d, got.Window1dStart, before.Window1dStart},
				{"7d", got.Usage7d, got.Window7dStart, before.Window7dStart},
			} {
				if other.name == window {
					s.Require().Zero(other.usage)
					s.Require().Nil(other.start)
				} else {
					s.Require().Equal(float64(42), other.usage, other.name)
					s.Require().NotNil(other.start)
					s.Require().Equal(other.beforeStart, other.start, other.name)
				}
			}
			s.Require().Equal(float64(9), got.QuotaUsed)
			s.Require().Equal("preserved", got.Name)
			s.Require().Equal(float64(100), got.Quota)
			s.Require().Equal(float64(100), got.RateLimit5h)
			s.Require().Equal(float64(200), got.RateLimit1d)
			s.Require().Equal(float64(300), got.RateLimit7d)
		})
	}
}

// api_keys 上的用量列由计费热路径原子递增（IncrementQuotaUsed /
// IncrementRateLimitUsage）。编辑 Key 时若整行回写，
// 并发累计的配额与限流计数就会被旧快照覆盖。

func (s *APIKeyRepoSuite) TestUpdate_DoesNotRevertConcurrentQuotaUsage() {
	user := s.mustCreateUser("apikey-lost-update-quota@example.com")
	key := &service.APIKey{
		UserID: user.ID,
		Key:    "sk-lost-update-quota",
		Name:   "before",
		Status: service.StatusActive,
		Quota:  100,
	}
	s.Require().NoError(s.repo.Create(s.ctx, key), "Create")

	stale, err := s.repo.GetByID(s.ctx, key.ID)
	s.Require().NoError(err, "GetByID")
	s.Require().Zero(stale.QuotaUsed)

	newUsed, err := s.repo.IncrementQuotaUsed(s.ctx, key.ID, 30)
	s.Require().NoError(err, "IncrementQuotaUsed")
	s.Require().InDelta(30, newUsed, 1e-9)

	stale.Name = "after"
	s.Require().NoError(
		s.repo.Update(s.ctx, stale, service.APIKeyUpdateFields{Name: true}),
		"Update",
	)

	got, err := s.repo.GetByID(s.ctx, key.ID)
	s.Require().NoError(err, "GetByID after update")
	s.Require().Equal("after", got.Name, "declared column must still be written")
	s.Require().InDelta(30, got.QuotaUsed, 1e-9, "quota_used must not be reverted by a stale key edit")
}

func (s *APIKeyRepoSuite) TestUpdate_DoesNotRevertConcurrentRateLimitUsage() {
	user := s.mustCreateUser("apikey-lost-update-ratelimit@example.com")
	key := &service.APIKey{
		UserID:      user.ID,
		Key:         "sk-lost-update-ratelimit",
		Name:        "before",
		Status:      service.StatusActive,
		RateLimit5h: 100,
	}
	s.Require().NoError(s.repo.Create(s.ctx, key), "Create")

	stale, err := s.repo.GetByID(s.ctx, key.ID)
	s.Require().NoError(err, "GetByID")
	s.Require().Zero(stale.Usage5h)

	s.Require().NoError(s.repo.IncrementRateLimitUsage(s.ctx, key.ID, 42), "IncrementRateLimitUsage")

	stale.Name = "after"
	s.Require().NoError(
		s.repo.Update(s.ctx, stale, service.APIKeyUpdateFields{Name: true}),
		"Update",
	)

	got, err := s.repo.GetByID(s.ctx, key.ID)
	s.Require().NoError(err, "GetByID after update")
	s.Require().InDelta(42, got.Usage5h, 1e-9, "usage_5h must not be reverted by a stale key edit")
	s.Require().InDelta(42, got.Usage1d, 1e-9, "usage_1d must not be reverted by a stale key edit")
	s.Require().InDelta(42, got.Usage7d, 1e-9, "usage_7d must not be reverted by a stale key edit")
}

// 显式重置仍然必须生效，避免收窄写入列时把功能改坏。
func (s *APIKeyRepoSuite) TestUpdate_StillResetsUsageWhenDeclared() {
	user := s.mustCreateUser("apikey-reset-usage@example.com")
	key := &service.APIKey{
		UserID: user.ID,
		Key:    "sk-reset-usage",
		Name:   "reset",
		Status: service.StatusActive,
		Quota:  100,
	}
	s.Require().NoError(s.repo.Create(s.ctx, key), "Create")

	_, err := s.repo.IncrementQuotaUsed(s.ctx, key.ID, 30)
	s.Require().NoError(err, "IncrementQuotaUsed")
	s.Require().NoError(s.repo.IncrementRateLimitUsage(s.ctx, key.ID, 42), "IncrementRateLimitUsage")

	current, err := s.repo.GetByID(s.ctx, key.ID)
	s.Require().NoError(err, "GetByID")
	current.QuotaUsed = 0
	current.Usage5h = 0
	current.Usage1d = 0
	current.Usage7d = 0
	current.Window5hStart = nil
	current.Window1dStart = nil
	current.Window7dStart = nil
	s.Require().NoError(
		s.repo.Update(s.ctx, current, service.APIKeyUpdateFields{QuotaUsed: true, RateLimitUsage: true}),
		"Update",
	)

	got, err := s.repo.GetByID(s.ctx, key.ID)
	s.Require().NoError(err, "GetByID after reset")
	s.Require().Zero(got.QuotaUsed, "explicit quota reset must still apply")
	s.Require().Zero(got.Usage5h, "explicit rate limit reset must still apply")
	s.Require().Zero(got.Usage1d)
	s.Require().Zero(got.Usage7d)
	s.Require().Nil(got.Window5hStart)
}
