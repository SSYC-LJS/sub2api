package handler

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUserMonitorViewToItemIncludesWindowStats(t *testing.T) {
	item := userMonitorViewToItem(&service.UserMonitorView{
		ID:             10,
		Name:           "VIP Group",
		Provider:       "openai",
		GroupName:      "VIP Group",
		PrimaryModel:   "gpt-4o",
		PrimaryStatus:  "operational",
		Availability7d: 99.5,
		WindowStats: service.GroupWindowStats{
			Requests1h:  11,
			Success1h:   10,
			Errors1h:    1,
			Requests12h: 120,
			Success12h:  118,
			Errors12h:   2,
			Requests24h: 240,
			Success24h:  235,
			Errors24h:   5,
		},
	})

	require.Equal(t, 11, item.WindowStats.Requests1h)
	require.Equal(t, 120, item.WindowStats.Requests12h)
	require.Equal(t, 240, item.WindowStats.Requests24h)

	encoded, err := json.Marshal(item)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"id":10,
		"name":"VIP Group",
		"provider":"openai",
		"group_name":"VIP Group",
		"primary_model":"gpt-4o",
		"primary_status":"operational",
		"primary_latency_ms":null,
		"primary_ping_latency_ms":null,
		"availability_7d":99.5,
		"window_stats":{
			"requests_1h":11,
			"success_1h":10,
			"errors_1h":1,
			"requests_12h":120,
			"success_12h":118,
			"errors_12h":2,
			"requests_24h":240,
			"success_24h":235,
			"errors_24h":5
		},
		"extra_models":[],
		"timeline":[]
	}`, string(encoded))
}
