package auth

import "testing"

// first_name and last_name are NOT NULL on the users table, but Google's ID
// token guarantees neither. Every shape below must still yield a storable pair.
func TestSplitName(t *testing.T) {
	tests := []struct {
		name                     string
		given, family, full, mail string
		wantFirst, wantLast      string
	}{
		{
			name: "given and family present",
			given: "Amina", family: "Ngassa", full: "Amina Ngassa", mail: "a@gmail.com",
			wantFirst: "Amina", wantLast: "Ngassa",
		},
		{
			name: "only a full name",
			full: "Amina Ngassa", mail: "a@gmail.com",
			wantFirst: "Amina", wantLast: "Ngassa",
		},
		{
			name: "multi-word full name splits on the last space",
			full: "Marie Claire Ngassa Etoundi", mail: "a@gmail.com",
			wantFirst: "Marie Claire Ngassa", wantLast: "Etoundi",
		},
		{
			name: "single-word full name",
			full: "Prince", mail: "p@gmail.com",
			wantFirst: "Prince", wantLast: "—",
		},
		{
			name: "given only",
			given: "Amina", mail: "a@gmail.com",
			wantFirst: "Amina", wantLast: "—",
		},
		{
			name: "family only still yields a first name from the address",
			family: "Ngassa", mail: "amina@gmail.com",
			wantFirst: "Amina", wantLast: "Ngassa",
		},
		{
			name: "nothing but an email — dots become spaces",
			mail: "amina.ngassa@gmail.com",
			wantFirst: "Amina ngassa", wantLast: "—",
		},
		{
			name: "email separators normalised",
			mail: "jean_pierre-b@gmail.com",
			wantFirst: "Jean pierre b", wantLast: "—",
		},
		{
			name:      "no usable input at all",
			wantFirst: "Member", wantLast: "—",
		},
		{
			name: "whitespace-only fields are ignored",
			given: "   ", family: "  ", full: " ", mail: "zoe@gmail.com",
			wantFirst: "Zoe", wantLast: "—",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			first, last := splitName(tc.given, tc.family, tc.full, tc.mail)
			if first != tc.wantFirst || last != tc.wantLast {
				t.Errorf("splitName(%q,%q,%q,%q) = (%q,%q), want (%q,%q)",
					tc.given, tc.family, tc.full, tc.mail, first, last, tc.wantFirst, tc.wantLast)
			}
			// The invariant the database depends on.
			if first == "" || last == "" {
				t.Error("neither name may be empty — both columns are NOT NULL")
			}
		})
	}
}
