package promotion

import (
	"testing"
	"time"
)

// "Live" is what decides whether a customer gets a discount, and it is false
// for four independent reasons. Getting any of them wrong either leaks a sale
// that should have ended or hides one that should be running.
func TestPromotionLive(t *testing.T) {
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	ptr := func(t time.Time) *time.Time { return &t }
	num := func(n int) *int { return &n }

	cases := []struct {
		name string
		p    Promotion
		want bool
	}{
		{
			name: "active, started, no expiry",
			p:    Promotion{IsActive: true, StartsAt: now.Add(-time.Hour)},
			want: true,
		},
		{
			name: "switched off",
			p:    Promotion{IsActive: false, StartsAt: now.Add(-time.Hour)},
			want: false,
		},
		{
			name: "scheduled for later",
			p:    Promotion{IsActive: true, StartsAt: now.Add(24 * time.Hour)},
			want: false,
		},
		{
			name: "expired",
			p:    Promotion{IsActive: true, StartsAt: now.Add(-48 * time.Hour), ExpiresAt: ptr(now.Add(-time.Hour))},
			want: false,
		},
		{
			name: "within window",
			p:    Promotion{IsActive: true, StartsAt: now.Add(-time.Hour), ExpiresAt: ptr(now.Add(time.Hour))},
			want: true,
		},
		{
			name: "usage cap reached",
			p:    Promotion{IsActive: true, StartsAt: now.Add(-time.Hour), MaxUses: num(10), UsesCount: 10},
			want: false,
		},
		{
			name: "usage cap not yet reached",
			p:    Promotion{IsActive: true, StartsAt: now.Add(-time.Hour), MaxUses: num(10), UsesCount: 9},
			want: true,
		},
		{
			name: "unlimited uses",
			p:    Promotion{IsActive: true, StartsAt: now.Add(-time.Hour), UsesCount: 9999},
			want: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.p.live(now); got != tc.want {
				t.Errorf("live() = %v, want %v", got, tc.want)
			}
		})
	}
}

// The boundary matters: a sale advertised as ending at midnight should still
// work at midnight, and one starting at noon should work at noon.
func TestPromotionLiveBoundaries(t *testing.T) {
	at := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)

	starting := Promotion{IsActive: true, StartsAt: at}
	if !starting.live(at) {
		t.Error("a promotion should be live at the instant it starts")
	}

	expiry := at
	ending := Promotion{IsActive: true, StartsAt: at.Add(-time.Hour), ExpiresAt: &expiry}
	if !ending.live(at) {
		t.Error("a promotion should still be live at the instant it expires")
	}
	if ending.live(at.Add(time.Nanosecond)) {
		t.Error("a promotion must not be live after its expiry")
	}
}
