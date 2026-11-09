//go:build integration

package repository

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// Exercise the public repository writes and both full and partial auth reads.
func (s *GroupRepoSuite) TestLocalGroupConfigRoundTrip() {
	fallback := &service.Group{Name: "local-config-fallback", Platform: service.PlatformOpenAI, RateMultiplier: 1, Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeStandard}
	s.Require().NoError(s.repo.Create(s.ctx, fallback))
	total := 123.45
	g := &service.Group{Name: "local-config", Platform: service.PlatformOpenAI, RateMultiplier: 1.25, Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeStandard,
		DisplayRateMultiplier: 1.75, UsageMultiplierEnabled: true, UsageMultiplier: 2.5, TotalLimitUSD: &total, QuotaFallbackGroupID: &fallback.ID, ModelPolicyMode: "force", ModelPolicyModel: "gpt-5.4"}
	s.Require().NoError(s.repo.Create(s.ctx, g))
	user := mustCreateUser(s.T(), s.tx.Client(), &service.User{Email: "local-group-config@test.com"})
	keys := newAPIKeyRepositoryWithSQL(s.tx.Client(), s.tx)
	key := &service.APIKey{UserID: user.ID, GroupID: &g.ID, Key: "sk-local-group-config", Name: "local-config", Status: service.StatusActive}
	s.Require().NoError(keys.Create(s.ctx, key))
	assertConfig := func(got, want *service.Group) {
		s.Require().NotNil(got)
		s.Equal(want.DisplayRateMultiplier, got.DisplayRateMultiplier)
		s.Equal(want.UsageMultiplierEnabled, got.UsageMultiplierEnabled)
		s.Equal(want.UsageMultiplier, got.UsageMultiplier)
		s.Equal(want.TotalLimitUSD, got.TotalLimitUSD)
		s.Equal(want.QuotaFallbackGroupID, got.QuotaFallbackGroupID)
		s.Equal(want.ModelPolicyMode, got.ModelPolicyMode)
		s.Equal(want.ModelPolicyModel, got.ModelPolicyModel)
	}
	checkReads := func() {
		got, err := s.repo.GetByID(s.ctx, g.ID)
		s.Require().NoError(err)
		assertConfig(got, g)
		full, err := keys.GetByID(s.ctx, key.ID)
		s.Require().NoError(err)
		assertConfig(full.Group, g)
		auth, err := keys.GetByKeyForAuth(s.ctx, key.Key)
		s.Require().NoError(err)
		assertConfig(auth.Group, g)
		groups, _, err := s.repo.List(s.ctx, pagination.PaginationParams{Page: 1, PageSize: 100})
		s.Require().NoError(err)
		found := false
		for i := range groups {
			if groups[i].ID == g.ID {
				assertConfig(&groups[i], g)
				found = true
			}
		}
		s.True(found)
	}
	checkReads()
	newTotal := 456.78
	g.DisplayRateMultiplier, g.UsageMultiplier = 3.25, 4.5
	g.TotalLimitUSD, g.QuotaFallbackGroupID = &newTotal, &fallback.ID
	g.ModelPolicyModel = "gpt-5.4-mini"
	s.Require().NoError(s.repo.Update(s.ctx, g))
	checkReads()

	// The transactional duplicate persistence entry must use the same builder.
	copy := *g
	copy.ID, copy.Name, copy.DuplicateOperationID = 0, "local-config-copy", "local-config-copy-op"
	s.Require().NoError(s.repo.CreateFromSource(s.ctx, &copy, g.ID))
	gotCopy, err := s.repo.FindByDuplicateOperationID(s.ctx, copy.DuplicateOperationID)
	s.Require().NoError(err)
	assertConfig(gotCopy, g)

	// Clearing optional limits/relay and disabling the multiplier must persist.
	g.TotalLimitUSD, g.QuotaFallbackGroupID = nil, nil
	g.UsageMultiplierEnabled = false
	g.DisplayRateMultiplier, g.UsageMultiplier = 1, 1
	g.ModelPolicyMode, g.ModelPolicyModel = "", ""
	s.Require().NoError(s.repo.Update(s.ctx, g))
	checkReads()
}

func (s *GroupRepoSuite) TestLocalGroupConfigDefaults() {
	g := &service.Group{Name: "local-config-defaults", Platform: service.PlatformOpenAI, RateMultiplier: 1, Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeStandard}
	s.Require().NoError(s.repo.Create(s.ctx, g))
	got, err := s.repo.GetByID(s.ctx, g.ID)
	s.Require().NoError(err)
	s.Equal(float64(1), got.DisplayRateMultiplier)
	s.Equal(float64(1), got.UsageMultiplier)
	s.False(got.UsageMultiplierEnabled)
	s.Nil(got.TotalLimitUSD)
	s.Nil(got.QuotaFallbackGroupID)
}
