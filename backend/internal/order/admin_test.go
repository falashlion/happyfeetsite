package order

import (
	"testing"
	"time"
)

// The transition map is the only thing standing between a dropdown and the
// order table, so it is worth asserting rather than eyeballing.
func TestCanTransition(t *testing.T) {
	cases := []struct {
		from, to string
		want     bool
	}{
		{"PLACED", "PROCESSING", true},
		{"PLACED", "DELIVERED", true}, // small shop, same-day hand delivery
		{"PROCESSING", "SHIPPED", true},
		{"SHIPPED", "DELIVERED", true},
		{"SHIPPED", "CANCELLED", true}, // refused at the door
		{"PAID", "CANCELLED", true},

		{"DELIVERED", "SHIPPED", false}, // terminal: no walking it back
		{"DELIVERED", "CANCELLED", false},
		{"CANCELLED", "PROCESSING", false},
		{"RETURNED", "DELIVERED", false},
		{"REFUNDED", "DELIVERED", false},
		{"SHIPPED", "PROCESSING", false}, // no reversing the fulfilment path
		{"PLACED", "RETURNED", false},    // returns have their own flow
		{"PLACED", "PAID", false},        // payment drives that, not the console
		{"", "DELIVERED", false},
		{"PLACED", "NOT_A_STATUS", false},
	}
	for _, c := range cases {
		if got := canTransition(c.from, c.to); got != c.want {
			t.Errorf("canTransition(%q,%q) = %v, want %v", c.from, c.to, got, c.want)
		}
	}
}

// Every target the map offers must be a real enum label, or the console would
// render a button whose request Postgres rejects with a 500.
func TestTransitionTargetsAreValidStatuses(t *testing.T) {
	for from, targets := range transitions {
		if !validStatuses[from] {
			t.Errorf("transition source %q is not a known order status", from)
		}
		for _, to := range targets {
			if !validStatuses[to] {
				t.Errorf("transition %q -> %q: target is not a known order status", from, to)
			}
			if to == from {
				t.Errorf("transition %q -> itself should not be listed", from)
			}
		}
	}
}

// NextStatuses feeds the console's dropdown; handing out the live slice would
// let a caller rewrite the state machine.
func TestNextStatusesCopies(t *testing.T) {
	got := NextStatuses("SHIPPED")
	if len(got) == 0 {
		t.Fatal("expected at least one move from SHIPPED")
	}
	got[0] = "TAMPERED"
	if transitions["SHIPPED"][0] == "TAMPERED" {
		t.Error("NextStatuses returned the backing array, not a copy")
	}

	if n := NextStatuses("DELIVERED"); len(n) != 0 {
		t.Errorf("DELIVERED is terminal, got %v", n)
	}
	if n := NextStatuses("NOT_A_STATUS"); len(n) != 0 {
		t.Errorf("unknown status should offer nothing, got %v", n)
	}
}

func TestCursorRoundTrip(t *testing.T) {
	// Nanosecond precision matters: two orders placed in the same millisecond
	// would otherwise collide and the page would repeat a row.
	at := time.Date(2026, 10, 10, 9, 15, 30, 123456789, time.UTC)
	const id = "7f1c9d4e-2b3a-4c5d-8e9f-0a1b2c3d4e5f"

	gotAt, gotID, err := decodeCursor(encodeCursor(at, id))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !gotAt.Equal(at) {
		t.Errorf("placed_at = %v, want %v", gotAt, at)
	}
	if gotID != id {
		t.Errorf("id = %q, want %q", gotID, id)
	}
}

func TestDecodeCursorRejectsGarbage(t *testing.T) {
	for _, in := range []string{
		"not base64 !!",
		"YWJj",               // valid base64, no separator
		"MjAyNi0xMC0xMHxpZA", // separator present, unparseable timestamp
		"",
	} {
		if _, _, err := decodeCursor(in); err == nil {
			t.Errorf("decodeCursor(%q) accepted a malformed cursor", in)
		}
	}
}
