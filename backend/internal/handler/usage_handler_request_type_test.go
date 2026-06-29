package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type userUsageRepoCapture struct {
	service.UsageLogRepository
	listParams           pagination.PaginationParams
	listFilters          usagestats.UsageLogFilters
	tokenRankingCalls    int
	spendingRankingCalls int
}

func (s *userUsageRepoCapture) ListWithFilters(ctx context.Context, params pagination.PaginationParams, filters usagestats.UsageLogFilters) ([]service.UsageLog, *pagination.PaginationResult, error) {
	s.listParams = params
	s.listFilters = filters
	return []service.UsageLog{}, &pagination.PaginationResult{
		Total:    0,
		Page:     params.Page,
		PageSize: params.PageSize,
		Pages:    0,
	}, nil
}

func (s *userUsageRepoCapture) GetUserTokenRanking(ctx context.Context, startTime, endTime time.Time, limit int) (*usagestats.UserSpendingRankingResponse, error) {
	s.tokenRankingCalls++
	return &usagestats.UserSpendingRankingResponse{
		Ranking: []usagestats.UserSpendingRankingItem{{
			UserID:     7,
			Email:      "seven@example.com",
			Username:   "SevenUser",
			ActualCost: 1.5,
			Requests:   3,
			Tokens:     7000,
		}},
		TotalActualCost: 1.5,
		TotalRequests:   3,
		TotalTokens:     7000,
	}, nil
}

func (s *userUsageRepoCapture) GetUserSpendingRanking(ctx context.Context, startTime, endTime time.Time, limit int) (*usagestats.UserSpendingRankingResponse, error) {
	s.spendingRankingCalls++
	return &usagestats.UserSpendingRankingResponse{
		Ranking: []usagestats.UserSpendingRankingItem{{
			UserID:     9,
			Email:      "spender@example.com",
			Username:   "SpenderUser",
			ActualCost: 12.5,
			Requests:   8,
			Tokens:     900,
		}},
		TotalActualCost: 12.5,
		TotalRequests:   8,
		TotalTokens:     900,
	}, nil
}

func newUserUsageRequestTypeTestRouter(repo *userUsageRepoCapture) *gin.Engine {
	gin.SetMode(gin.TestMode)
	usageSvc := service.NewUsageService(repo, nil, nil, nil)
	handler := NewUsageHandler(usageSvc, nil, nil, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42})
		c.Next()
	})
	router.GET("/usage", handler.List)
	router.GET("/usage/dashboard/ranking", handler.DashboardRanking)
	return router
}

func TestUserUsageListRequestTypePriority(t *testing.T) {
	repo := &userUsageRepoCapture{}
	router := newUserUsageRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/usage?request_type=ws_v2&stream=bad", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(42), repo.listFilters.UserID)
	require.NotNil(t, repo.listFilters.RequestType)
	require.Equal(t, int16(service.RequestTypeWSV2), *repo.listFilters.RequestType)
	require.Nil(t, repo.listFilters.Stream)
}

func TestUserUsageListInvalidRequestType(t *testing.T) {
	repo := &userUsageRepoCapture{}
	router := newUserUsageRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/usage?request_type=invalid", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUserUsageListInvalidStream(t *testing.T) {
	repo := &userUsageRepoCapture{}
	router := newUserUsageRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/usage?stream=invalid", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUserDashboardRankingSortByCostMasksIdentities(t *testing.T) {
	repo := &userUsageRepoCapture{}
	router := newUserUsageRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/usage/dashboard/ranking?sort_by=cost&timezone=UTC", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 0, repo.tokenRankingCalls)
	require.Equal(t, 4, repo.spendingRankingCalls)

	var body struct {
		Data map[string]struct {
			Ranking []struct {
				Email    string `json:"email"`
				Username string `json:"username"`
			} `json:"ranking"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.NotEmpty(t, body.Data["all"].Ranking)
	item := body.Data["all"].Ranking[0]
	require.Contains(t, item.Email, "**")
	require.Contains(t, item.Username, "**")
	require.NotContains(t, rec.Body.String(), "spender@example.com")
	require.NotContains(t, rec.Body.String(), "SpenderUser")
}

func TestUserDashboardRankingInvalidSortBy(t *testing.T) {
	repo := &userUsageRepoCapture{}
	router := newUserUsageRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/usage/dashboard/ranking?sort_by=balance", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, 0, repo.tokenRankingCalls)
	require.Equal(t, 0, repo.spendingRankingCalls)
}
