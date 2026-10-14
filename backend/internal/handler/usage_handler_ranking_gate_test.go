package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// rankingGateSettingRepoStub 只实现 IsUsageRankingEnabled 用到的 GetMultiple。
type rankingGateSettingRepoStub struct {
	service.SettingRepository
	values map[string]string
	err    error
}

func (s *rankingGateSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	if s.err != nil {
		return nil, s.err
	}
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

type rankingGateUsageRepoStub struct {
	service.UsageLogRepository
	calls int
}

func (s *rankingGateUsageRepoStub) GetUserTokenRanking(context.Context, time.Time, time.Time, int) (*usagestats.UserSpendingRankingResponse, error) {
	s.calls++
	return &usagestats.UserSpendingRankingResponse{}, nil
}

func (s *rankingGateUsageRepoStub) GetUserSpendingRanking(context.Context, time.Time, time.Time, int) (*usagestats.UserSpendingRankingResponse, error) {
	s.calls++
	return &usagestats.UserSpendingRankingResponse{}, nil
}

func (s *rankingGateUsageRepoStub) GetUserBalanceRedeemRanking(context.Context, time.Time, time.Time, int) (*usagestats.UserRedeemRankingResponse, error) {
	s.calls++
	return &usagestats.UserRedeemRankingResponse{}, nil
}

func newRankingGateTestRouter(settingRepo service.SettingRepository, usageRepo *rankingGateUsageRepoStub) *gin.Engine {
	gin.SetMode(gin.TestMode)
	usageSvc := service.NewUsageService(usageRepo, nil, nil, nil)
	settingSvc := service.NewSettingService(settingRepo, &config.Config{})
	handler := NewUsageHandler(usageSvc, nil, nil, settingSvc)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42})
		c.Next()
	})
	router.GET("/usage/dashboard/ranking", handler.DashboardRanking)
	return router
}

// 管理员把「用量排行榜」关掉后，接口必须 404，且不再查询排行榜数据。
func TestDashboardRankingReturns404WhenUsageRankingDisabled(t *testing.T) {
	usageRepo := &rankingGateUsageRepoStub{}
	router := newRankingGateTestRouter(&rankingGateSettingRepoStub{
		values: map[string]string{service.SettingKeyUsageRankingEnabled: "false"},
	}, usageRepo)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/usage/dashboard/ranking", nil))

	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Zero(t, usageRepo.calls, "开关关闭后不应再查询排行榜数据")
}

// opt-out 开关：缺省（老部署没有这一行）与读取失败都保持可用。
func TestDashboardRankingStaysAvailableWhenSwitchMissingOrUnreadable(t *testing.T) {
	for _, tc := range []struct {
		name string
		repo service.SettingRepository
	}{
		{name: "missing row keeps the feature on", repo: &rankingGateSettingRepoStub{values: map[string]string{}}},
		{name: "explicit true", repo: &rankingGateSettingRepoStub{values: map[string]string{service.SettingKeyUsageRankingEnabled: "true"}}},
		{name: "read error fails open", repo: &rankingGateSettingRepoStub{err: errors.New("settings database unavailable")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			usageRepo := &rankingGateUsageRepoStub{}
			router := newRankingGateTestRouter(tc.repo, usageRepo)

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/usage/dashboard/ranking", nil))

			require.Equal(t, http.StatusOK, rec.Code)
			require.Positive(t, usageRepo.calls)
		})
	}
}
