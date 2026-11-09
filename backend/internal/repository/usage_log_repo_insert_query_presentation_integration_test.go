//go:build integration

package repository

import (
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// Exercise real PostgreSQL INSERTs and the shared query scanner. Group settings
// deliberately disagree with the captured snapshot: reads must not recompute it.
func (s *UsageLogRepoSuite) TestPresentationSnapshotRoundTrip() {
	user := mustCreateUser(s.T(), s.client, &service.User{Email: "snapshot@test.com"})
	key := mustCreateApiKey(s.T(), s.client, &service.APIKey{UserID: user.ID, Key: "sk-snapshot", Name: "k"})
	account := mustCreateAccount(s.T(), s.client, &service.Account{Name: "snapshot"})
	group := mustCreateGroup(s.T(), s.client, &service.Group{Name: "snapshot"})
	_, err := s.repo.sql.ExecContext(s.ctx, "UPDATE groups SET usage_multiplier_enabled = true, usage_multiplier = 9 WHERE id = $1", group.ID)
	s.Require().NoError(err)

	for _, path := range []string{"single", "batch", "best-effort", "settlement"} {
		for _, snapshot := range []float64{2.5, 0, -1} {
			log := &service.UsageLog{UserID: user.ID, APIKeyID: key.ID, AccountID: account.ID, GroupID: &group.ID, RequestID: fmt.Sprintf("snapshot-%s-%g", path, snapshot), Model: "claude-3", InputTokens: 1000, CacheCreation30mTokens: 101, CacheCreation12hTokens: 203, ActualCost: 1, PresentationMultiplier: snapshot}
			switch path {
			case "single":
				_, err = s.repo.createSingle(s.ctx, s.repo.sql, log)
			case "batch":
				query, args := buildUsageLogBatchInsertQuery([]string{"key"}, map[string]usageLogInsertPrepared{"key": prepareUsageLogInsert(log)})
				result, queryErr := s.repo.sql.QueryContext(s.ctx, query, args...)
				err = queryErr
				if queryErr == nil {
					for result.Next() {
					}
					err = result.Err()
					s.Require().NoError(result.Close())
				}
			case "best-effort":
				query, args := buildUsageLogBestEffortInsertQuery([]usageLogInsertPrepared{prepareUsageLogInsert(log)})
				_, err = s.repo.sql.ExecContext(s.ctx, query, args...)
			case "settlement":
				err = s.repo.CreateSettlement(s.ctx, log)
			}
			s.Require().NoError(err, path)
			result, queryErr := s.repo.sql.QueryContext(s.ctx, "SELECT "+usageLogSelectColumns+" FROM usage_logs WHERE request_id = $1 AND api_key_id = $2", log.RequestID, key.ID)
			s.Require().NoError(queryErr)
			s.Require().True(result.Next())
			got, scanErr := scanUsageLog(result)
			s.Require().NoError(scanErr)
			s.Require().NoError(result.Close())
			want := snapshot
			if want <= 0 {
				want = 1
			}
			s.Require().Equal(want, got.PresentationMultiplier, path)
			s.Require().Equal(101, got.CacheCreation30mTokens, path)
			s.Require().Equal(203, got.CacheCreation12hTokens, path)
			// A settled retry cannot replace the historical snapshot.
			log.PresentationMultiplier = 7
			log.CacheCreation30mTokens, log.CacheCreation12hTokens = 999, 999
			inserted, duplicateErr := s.repo.createSingle(s.ctx, s.repo.sql, log)
			s.Require().NoError(duplicateErr)
			s.Require().False(inserted)
			s.Require().NoError(s.repo.CreateSettlement(s.ctx, log))
			stored, getErr := s.repo.GetByID(s.ctx, got.ID)
			s.Require().NoError(getErr)
			s.Require().Equal(want, stored.PresentationMultiplier)
			s.Require().Equal(101, stored.CacheCreation30mTokens)
			s.Require().Equal(203, stored.CacheCreation12hTokens)
		}
	}
	// Changing the group later must not rewrite already recorded snapshots.
	_, err = s.repo.sql.ExecContext(s.ctx, "UPDATE groups SET usage_multiplier = 11 WHERE id = $1", group.ID)
	s.Require().NoError(err)
	result, err := s.repo.sql.QueryContext(s.ctx, "SELECT "+usageLogSelectColumns+" FROM usage_logs WHERE group_id = $1", group.ID)
	s.Require().NoError(err)
	count := 0
	for result.Next() {
		got, scanErr := scanUsageLog(result)
		s.Require().NoError(scanErr)
		s.Require().Contains([]float64{1, 2.5}, got.PresentationMultiplier)
		count++
	}
	s.Require().NoError(result.Err())
	s.Require().NoError(result.Close())
	s.Require().Equal(12, count)
	// A failed placeholder is upgraded with the successful request's snapshot.
	log := &service.UsageLog{UserID: user.ID, APIKeyID: key.ID, AccountID: account.ID, RequestID: "snapshot-upgrade", Model: "claude-3"}
	_, err = s.repo.createSingle(s.ctx, s.repo.sql, log)
	s.Require().NoError(err)
	log.ActualCost, log.PresentationMultiplier = 1, 3.25
	s.Require().NoError(s.repo.CreateSettlement(s.ctx, log))
	got, err := s.repo.GetByID(s.ctx, log.ID)
	s.Require().NoError(err)
	s.Require().Equal(3.25, got.PresentationMultiplier)
}

func (s *UsageLogRepoSuite) TestCacheTTLSettlementUpgrade() {
	user := mustCreateUser(s.T(), s.client, &service.User{Email: "ttl-settlement@test.com"})
	key := mustCreateApiKey(s.T(), s.client, &service.APIKey{UserID: user.ID, Key: "sk-ttl-settlement", Name: "k"})
	account := mustCreateAccount(s.T(), s.client, &service.Account{Name: "ttl-settlement"})
	for _, path := range []string{"settlement", "best-effort"} {
		log := &service.UsageLog{UserID: user.ID, APIKeyID: key.ID, AccountID: account.ID, RequestID: "ttl-upgrade-" + path, Model: "claude-3", CacheCreation30mTokens: 3, CacheCreation12hTokens: 4}
		_, err := s.repo.createSingle(s.ctx, s.repo.sql, log)
		s.Require().NoError(err)
		for _, step := range []struct {
			name                         string
			cost, snapshot, wantSnapshot float64
			cache30m, cache12h           int
			want30m, want12h             int
		}{
			{"unsettled retry", 0, 2.5, 1, 101, 203, 3, 4},
			{"upgrade", 1, 2.5, 2.5, 101, 203, 101, 203},
			{"settled retry", 2, 7, 2.5, 999, 999, 101, 203},
			{"no downgrade", 0, 1, 2.5, 0, 0, 101, 203},
		} {
			log.ActualCost, log.PresentationMultiplier = step.cost, step.snapshot
			log.CacheCreation30mTokens, log.CacheCreation12hTokens = step.cache30m, step.cache12h
			if path == "settlement" {
				err = s.repo.CreateSettlement(s.ctx, log)
			} else {
				query, args := buildUsageLogBestEffortInsertQuery([]usageLogInsertPrepared{prepareUsageLogInsert(log)})
				_, err = s.repo.sql.ExecContext(s.ctx, query, args...)
			}
			s.Require().NoError(err, path+" "+step.name)
			got, getErr := s.repo.GetByID(s.ctx, log.ID)
			s.Require().NoError(getErr)
			s.Require().Equal(step.wantSnapshot, got.PresentationMultiplier, path+" "+step.name)
			s.Require().Equal(step.want30m, got.CacheCreation30mTokens, path+" "+step.name)
			s.Require().Equal(step.want12h, got.CacheCreation12hTokens, path+" "+step.name)
		}
	}
}
