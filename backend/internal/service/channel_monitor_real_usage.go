package service

import (
	"context"
	"fmt"
	"sort"
)

// ListUserRealUsageView 基于当前用户可见分组与站内真实请求日志生成用户侧渠道状态。
func (s *ChannelMonitorService) ListUserRealUsageView(ctx context.Context, groups []Group) ([]*UserMonitorView, error) {
	if len(groups) == 0 {
		return []*UserMonitorView{}, nil
	}
	groupIDs := make([]int64, 0, len(groups))
	for i := range groups {
		if groups[i].ID > 0 {
			groupIDs = append(groupIDs, groups[i].ID)
		}
	}
	stats, err := s.repo.ListRealUsageGroupMonitorStats(ctx, groupIDs)
	if err != nil {
		return nil, fmt.Errorf("list real usage group monitor stats: %w", err)
	}

	views := make([]*UserMonitorView, 0, len(groups))
	for i := range groups {
		views = append(views, buildUserRealUsageView(groups[i], stats[groups[i].ID]))
	}
	sort.SliceStable(views, func(i, j int) bool {
		return views[i].ID < views[j].ID
	})
	return views, nil
}

func buildUserRealUsageView(group Group, stat *RealUsageGroupMonitorStat) *UserMonitorView {
	view := &UserMonitorView{
		ID:              group.ID,
		Name:            group.Name,
		Provider:        group.Platform,
		GroupName:       group.Name,
		PrimaryStatus:   "",
		Availability12h: 100,
		Timeline:        []UserMonitorTimelinePoint{},
		ExtraModels:     []ExtraModelStatus{},
	}
	if stat == nil {
		return view
	}

	view.PrimaryModel = stat.PrimaryModel
	view.PrimaryStatus = stat.PrimaryStatus
	view.PrimaryLatencyMs = stat.LatencyMs
	view.Availability12h = stat.Availability12h
	view.WindowStats = stat.WindowStats
	view.Timeline = stat.Timeline
	return view
}

// GetUserRealUsageDetail 基于真实请求日志返回某个可见分组的模型维度健康详情。
func (s *ChannelMonitorService) GetUserRealUsageDetail(ctx context.Context, group Group) (*UserMonitorDetail, error) {
	detail, err := s.repo.GetRealUsageGroupMonitorDetail(ctx, group.ID)
	if err != nil {
		return nil, fmt.Errorf("get real usage group monitor detail: %w", err)
	}
	models := []ModelDetail{}
	if detail != nil {
		models = detail.Models
	}
	return &UserMonitorDetail{
		ID:        group.ID,
		Name:      group.Name,
		Provider:  group.Platform,
		GroupName: group.Name,
		Models:    models,
	}, nil
}
