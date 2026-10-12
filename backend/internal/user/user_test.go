package user

import (
	"strings"
	"testing"
)

func ptr(s string) *string { return &s }

// The regression this guards: PATCH used to take plain strings and write every
// column unconditionally, so setting one field blanked the others. A patch must
// name only the columns it was given.
func TestBuildUpdateTouchesOnlyWhatWasSent(t *testing.T) {
	cases := []struct {
		name    string
		patch   ProfilePatch
		touches []string
		leaves  []string
	}{
		{
			name:    "first name only",
			patch:   ProfilePatch{FirstName: ptr("Felix")},
			touches: []string{"first_name"},
			leaves:  []string{"last_name", "phone", "phone_verified_at"},
		},
		{
			name:    "phone only",
			patch:   ProfilePatch{Phone: ptr("+237654904707")},
			touches: []string{"phone", "phone_verified_at"},
			leaves:  []string{"first_name", "last_name"},
		},
		{
			name:    "both names",
			patch:   ProfilePatch{FirstName: ptr("Felix"), LastName: ptr("Forchu")},
			touches: []string{"first_name", "last_name"},
			leaves:  []string{"phone"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sql, args := buildUpdate("u-1", c.patch)
			sets := sql[strings.Index(sql, "SET ")+4 : strings.Index(sql, " WHERE")]

			for _, col := range c.touches {
				if !strings.Contains(sets, col+"=") && !strings.Contains(sets, col+" =") {
					t.Errorf("expected %s to be set, got: %s", col, sets)
				}
			}
			for _, col := range c.leaves {
				if strings.Contains(sets, col+"=") || strings.Contains(sets, col+" =") {
					t.Errorf("%s was not in the patch but the statement writes it: %s", col, sets)
				}
			}
			if args[0] != "u-1" {
				t.Errorf("args[0] = %v, want the user id", args[0])
			}
			// Soft-deleted accounts must not be writable.
			if !strings.Contains(sql, "deleted_at IS NULL") {
				t.Errorf("update is missing the deleted_at guard: %s", sql)
			}
			if !strings.Contains(sets, "updated_at=NOW()") {
				t.Errorf("update does not stamp updated_at: %s", sets)
			}
		})
	}
}

// Every value has to travel as a bind parameter; a name with a quote in it is
// an ordinary name, not a syntax error.
func TestBuildUpdateBindsValues(t *testing.T) {
	sql, args := buildUpdate("u-1", ProfilePatch{
		FirstName: ptr("O'Brien"),
		Phone:     ptr("+237654904707"),
	})
	if strings.Contains(sql, "O'Brien") || strings.Contains(sql, "+237654904707") {
		t.Errorf("values were inlined into the statement: %s", sql)
	}
	if len(args) != 3 {
		t.Fatalf("args = %v, want id + 2 values", args)
	}
	if args[1] != "O'Brien" || args[2] != "+237654904707" {
		t.Errorf("args = %v, want the patched values in order", args)
	}
}

// Re-sending the number already on file must not quietly revoke the OTP
// verification that lets the account sign in by phone.
func TestBuildUpdateKeepsVerificationWhenPhoneIsUnchanged(t *testing.T) {
	sql, _ := buildUpdate("u-1", ProfilePatch{Phone: ptr("+237654904707")})
	if !strings.Contains(sql, "phone IS DISTINCT FROM") {
		t.Errorf("phone_verified_at is cleared unconditionally: %s", sql)
	}
	if !strings.Contains(sql, "ELSE phone_verified_at END") {
		t.Errorf("unchanged number does not keep its verification: %s", sql)
	}
}

// Postgres deduces one type per placeholder. The phone parameter is used as an
// assignment to a varchar column and inside a comparison, and dropping either
// cast makes the server reject the statement with SQLSTATE 42P08 — a 500 on
// every phone change, which is how this was found.
func TestBuildUpdateCastsThePhoneParameterConsistently(t *testing.T) {
	sql, _ := buildUpdate("u-1", ProfilePatch{Phone: ptr("+237654904707")})
	if n := strings.Count(sql, "$2::text"); n != 2 {
		t.Errorf("phone placeholder is cast %d time(s), want 2 (assignment and comparison): %s", n, sql)
	}
}

func TestEmptyPatch(t *testing.T) {
	if !(ProfilePatch{}).Empty() {
		t.Error("a patch with no fields should be empty")
	}
	if (ProfilePatch{LastName: ptr("Forchu")}).Empty() {
		t.Error("a patch with a field should not be empty")
	}
}

func TestNormalise(t *testing.T) {
	t.Run("trims", func(t *testing.T) {
		p := ProfilePatch{FirstName: ptr("  Felix  "), Phone: ptr(" +237654904707 ")}
		if err := p.Normalise(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if *p.FirstName != "Felix" {
			t.Errorf("first_name = %q, want %q", *p.FirstName, "Felix")
		}
		if *p.Phone != "+237654904707" {
			t.Errorf("phone = %q, want it trimmed", *p.Phone)
		}
	})

	// An explicit "" is a different request from omitting the field, and it is
	// one we refuse: a person has a name, and the old behaviour of silently
	// storing a blank is what made this worth fixing.
	for _, c := range []struct {
		name  string
		patch ProfilePatch
		want  string
	}{
		{"empty first name", ProfilePatch{FirstName: ptr("")}, "first_name"},
		{"whitespace last name", ProfilePatch{LastName: ptr("   ")}, "last_name"},
		{"cleared phone", ProfilePatch{Phone: ptr("")}, "phone"},
	} {
		t.Run(c.name, func(t *testing.T) {
			err := c.patch.Normalise()
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error %q does not name %s", err, c.want)
			}
		})
	}

	t.Run("omitted fields are fine", func(t *testing.T) {
		p := ProfilePatch{}
		if err := p.Normalise(); err != nil {
			t.Errorf("an empty patch should normalise cleanly, got %v", err)
		}
	})
}
