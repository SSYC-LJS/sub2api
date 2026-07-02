package handler

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

func TestMaskUserRankingIdentitiesPrefersUsernameAndPreservesEmailDomain(t *testing.T) {
	items := []usagestats.UserSpendingRankingItem{
		{UserID: 1, Email: "595341366@qq.com", Username: ""},
		{UserID: 2, Email: "724249023@qq.com", Username: "李文科"},
	}

	masked := maskUserRankingIdentities(items)

	if got, want := masked[0].Email, "595****366@qq.com"; got != want {
		t.Fatalf("email fallback = %q, want %q", got, want)
	}
	if got, want := masked[0].Username, ""; got != want {
		t.Fatalf("empty username should remain empty, got %q", got)
	}
	if got, want := masked[1].Username, "李****科"; got != want {
		t.Fatalf("username display = %q, want %q", got, want)
	}
	if got, want := masked[1].Email, ""; got != want {
		t.Fatalf("email should be hidden when username exists, got %q", got)
	}
}
