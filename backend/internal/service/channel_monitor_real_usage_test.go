package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildUserRealUsageViewDefaultsAvailabilityTo100WithoutStats(t *testing.T) {
	view := buildUserRealUsageView(Group{ID: 7, Name: "No Traffic", Platform: "openai"}, nil)

	require.Equal(t, float64(100), view.Availability12h)
}

func TestBuildUserRealUsageViewPreservesMeasuredZeroAvailability(t *testing.T) {
	view := buildUserRealUsageView(
		Group{ID: 8, Name: "Failed Traffic", Platform: "openai"},
		&RealUsageGroupMonitorStat{GroupID: 8, Availability12h: 0},
	)

	require.Equal(t, float64(0), view.Availability12h)
}
