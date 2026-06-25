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
		group := groups[i]
		view := &UserMonitorView{
			ID:            group.ID,
			Name:          group.Name,
			Provider:      group.Platform,
			GroupName:     group.Name,
			PrimaryStatus: "",
			Timeline:      []UserMonitorTimelinePoint{},
			ExtraModels:   []ExtraModelStatus{},
		}
		if stat := stats[group.ID]; stat != nil {
			view.PrimaryModel = stat.PrimaryModel
			view.PrimaryStatus = stat.PrimaryStatus
			view.PrimaryLatencyMs = stat.LatencyMs
			view.Availability7d = stat.Availability7d
			view.WindowStats = stat.WindowStats
			view.Timeline = stat.Timeline
		}
		views = append(views, view)
	}
	sort.SliceStable(views, func(i, j int) bool {
		return views[i].ID < views[j].ID
	})
	return views, nil
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
