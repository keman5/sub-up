package admin

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func TestRefreshListedAccountUsageWindowsRefreshesEachAccountAndContinuesAfterFailure(t *testing.T) {
	accounts := []service.Account{{ID: 11}, {ID: 12}, {ID: 13}}
	var calls []struct {
		id    int64
		force bool
	}

	refreshListedAccountUsageWindows(context.Background(), accounts, func(_ context.Context, id int64, force bool) (*service.UsageInfo, error) {
		calls = append(calls, struct {
			id    int64
			force bool
		}{id: id, force: force})
		if id == 12 {
			return nil, errors.New("upstream unavailable")
		}
		return &service.UsageInfo{}, nil
	})

	if len(calls) != len(accounts) {
		t.Fatalf("refresh calls = %d, want %d", len(calls), len(accounts))
	}
	for index, call := range calls {
		if call.id != accounts[index].ID || !call.force {
			t.Fatalf("call[%d] = %+v, want account %d with force=true", index, call, accounts[index].ID)
		}
	}
}

type listUsageRefreshAccountRepo struct {
	service.AccountRepository
	account *service.Account
	calls   []int64
}

func (r *listUsageRefreshAccountRepo) GetByID(_ context.Context, id int64) (*service.Account, error) {
	r.calls = append(r.calls, id)
	return r.account, nil
}

func TestAccountListRefreshUsageQueryRefreshesOnlyWhenRequested(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminService := newStubAdminService()
	adminService.accounts = []service.Account{{
		ID:        21,
		Name:      "api-key",
		Platform:  service.PlatformOpenAI,
		Type:      service.AccountTypeAPIKey,
		Status:    service.StatusActive,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}}
	repo := &listUsageRefreshAccountRepo{account: &adminService.accounts[0]}
	usageService := service.NewAccountUsageService(
		repo, nil, nil, nil, nil, nil, nil, nil,
		service.NewUsageCache(), nil, nil,
	)
	handler := NewAccountHandler(adminService, nil, nil, nil, nil, nil, nil, usageService, nil, nil, nil, nil, nil, nil)
	router := gin.New()
	router.GET("/api/v1/admin/accounts", handler.List)

	withoutRefresh := httptest.NewRecorder()
	router.ServeHTTP(withoutRefresh, httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts", nil))
	if withoutRefresh.Code != http.StatusOK {
		t.Fatalf("list without refresh status = %d, want %d", withoutRefresh.Code, http.StatusOK)
	}
	if len(repo.calls) != 0 {
		t.Fatalf("usage calls without refresh = %v, want none", repo.calls)
	}

	withRefresh := httptest.NewRecorder()
	router.ServeHTTP(withRefresh, httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts?refresh_usage=true", nil))
	if withRefresh.Code != http.StatusOK {
		t.Fatalf("list with refresh status = %d, want %d", withRefresh.Code, http.StatusOK)
	}
	if len(repo.calls) != 1 || repo.calls[0] != 21 {
		t.Fatalf("usage calls with refresh = %v, want [21]", repo.calls)
	}
}
