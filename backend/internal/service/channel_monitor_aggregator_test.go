package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildStatusSummaryDefaultsTwelveHourAvailabilityToOneHundred(t *testing.T) {
	summary := buildStatusSummary(nil, nil, "gpt-test", nil)

	require.Equal(t, float64(100), summary.Availability12h)
}

func TestBuildStatusSummaryUsesTwelveHourAvailabilitySample(t *testing.T) {
	summary := buildStatusSummary(
		nil,
		map[string]*ChannelMonitorAvailability{
			"gpt-test": {Model: "gpt-test", WindowHours: 12, AvailabilityPct: 87.5},
		},
		"gpt-test",
		nil,
	)

	require.Equal(t, 87.5, summary.Availability12h)
}

func TestMergeModelDetailsDefaultsTwelveHourAvailabilityToOneHundred(t *testing.T) {
	details := mergeModelDetails(
		&ChannelMonitor{PrimaryModel: "gpt-test"},
		nil,
		map[int]map[string]*ChannelMonitorAvailability{},
	)

	require.Len(t, details, 1)
	require.Equal(t, float64(100), details[0].Availability12h)
}
