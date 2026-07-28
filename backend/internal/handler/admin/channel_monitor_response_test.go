package admin

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestChannelMonitorToResponseDefaultsTwelveHourAvailabilityToOneHundred(t *testing.T) {
	response := channelMonitorToResponse(&service.ChannelMonitor{ID: 42, PrimaryModel: "gpt-test"})

	require.NotNil(t, response)
	require.Equal(t, float64(100), response.Availability12h)
}

func TestBuildListItemResponsePreservesMeasuredZeroAvailability(t *testing.T) {
	response := buildListItemResponse(
		&service.ChannelMonitor{ID: 42, PrimaryModel: "gpt-test"},
		service.MonitorStatusSummary{Availability12h: 0},
	)

	require.NotNil(t, response)
	require.Equal(t, float64(0), response.Availability12h)
}
