package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSystemHasBillingAttributionBlock(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{
			name: "genuine proxied claude code payload",
			body: `{"system":[{"type":"text","text":"x-anthropic-billing-header: cc_version=2.1.0; cc_entrypoint=cli;"}]}`,
			want: true,
		},
		{
			name: "prompt text without billing attribution",
			body: `{"system":[{"type":"text","text":"You are Claude Code."}]}`,
		},
		{
			name: "billing prefix without entrypoint",
			body: `{"system":[{"type":"text","text":"x-anthropic-billing-header: cc_version=2.1.0;"}]}`,
		},
		{
			name: "string system is not a genuine billing block",
			body: `{"system":"x-anthropic-billing-header: cc_entrypoint=cli;"}`,
		},
		{
			name: "invalid json",
			body: `{`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, systemHasBillingAttributionBlock([]byte(tt.body)))
		})
	}
}
