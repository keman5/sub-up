package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 捕获 ListUsers 入参、返回一个已删用户的 admin service 桩。
type searchUsersAdminStub struct {
	service.AdminService
	gotFilters service.UserListFilters
}

func (s *searchUsersAdminStub) ListUsers(ctx context.Context, page, pageSize int, filters service.UserListFilters, sortBy, sortOrder string) ([]service.User, int64, error) {
	s.gotFilters = filters
	ts := time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC)
	return []service.User{
		{ID: 1, Email: "active@test.com", Username: "active-user", Notes: "vip"},
		{ID: 2, Email: "deleted@test.com", Username: "deleted-user", Notes: "left", DeletedAt: &ts},
	}, 2, nil
}

func TestAdminUsageSearchUsers_IncludesDeletedAndFlags(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &searchUsersAdminStub{}
	handler := NewUsageHandler(nil, nil, stub, nil)
	router := gin.New()
	router.GET("/admin/usage/search-users", handler.SearchUsers)

	req := httptest.NewRequest(http.MethodGet, "/admin/usage/search-users?q=test", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, stub.gotFilters.IncludeDeleted, "SearchUsers 必须请求 IncludeDeleted")

	var resp struct {
		Data []struct {
			ID       int64  `json:"id"`
			Email    string `json:"email"`
			Username string `json:"username"`
			Notes    string `json:"notes"`
			Deleted  bool   `json:"deleted"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Data, 2)
	require.Equal(t, "active-user", resp.Data[0].Username)
	require.Equal(t, "vip", resp.Data[0].Notes)
	require.False(t, resp.Data[0].Deleted)
	require.Equal(t, "deleted-user", resp.Data[1].Username)
	require.Equal(t, "left", resp.Data[1].Notes)
	require.True(t, resp.Data[1].Deleted, "已删用户必须标记 deleted=true")
}

func TestAdminUsageSearchUsers_EmptyKeywordStillReturnsDefaultSuggestions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &searchUsersAdminStub{}
	handler := NewUsageHandler(nil, nil, stub, nil)
	router := gin.New()
	router.GET("/admin/usage/search-users", handler.SearchUsers)

	req := httptest.NewRequest(http.MethodGet, "/admin/usage/search-users?q=", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "", stub.gotFilters.Search, "空关键词应沿用默认用户建议查询，而不是直接短路返回")
	require.NotNil(t, stub.gotFilters.IncludeSubscriptions, "搜索建议接口应明确关闭订阅预加载")
	require.False(t, *stub.gotFilters.IncludeSubscriptions, "搜索建议接口不应预加载订阅，避免放大查询并引入 500 风险")

	var resp struct {
		Data []struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Data, 2)
}
