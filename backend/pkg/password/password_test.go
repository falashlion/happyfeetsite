package password

import (
	"strings"
	"testing"
)

func TestHashAndVerify(t *testing.T) {
	cases := []string{"SecurePass1", "hunter2", "CorrectHorseBatteryStaple!", ""}
	for _, pw := range cases {
		h, err := Hash(pw)
		if err != nil {
			t.Fatalf("Hash(%q): %v", pw, err)
		}
		if !strings.HasPrefix(h, "pbkdf2$") {
			t.Fatalf("Hash(%q): unexpected format: %s", pw, h)
		}
		if err := Verify(pw, h); err != nil {
			t.Fatalf("Verify(%q): %v", pw, err)
		}
	}
}

func TestVerifyWrongPassword(t *testing.T) {
	h, _ := Hash("correct")
	if err := Verify("wrong", h); err == nil {
		t.Fatal("expected mismatch error for wrong password")
	}
}

func TestVerifyBadFormat(t *testing.T) {
	if err := Verify("x", "not-a-valid-hash"); err == nil {
		t.Fatal("expected error for bad format")
	}
}

func TestTwoHashesDiffer(t *testing.T) {
	h1, _ := Hash("same")
	h2, _ := Hash("same")
	if h1 == h2 {
		t.Fatal("two hashes of same password should differ (random salt)")
	}
	if err := Verify("same", h1); err != nil {
		t.Fatal("verify h1 failed")
	}
	if err := Verify("same", h2); err != nil {
		t.Fatal("verify h2 failed")
	}
}

func BenchmarkHash(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = Hash("BenchmarkPassword1!")
	}
}
