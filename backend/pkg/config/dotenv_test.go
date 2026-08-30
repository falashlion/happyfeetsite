package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseEnvLine(t *testing.T) {
	tests := []struct {
		name           string
		line           string
		wantKey, wantV string
		wantOK         bool
	}{
		{"simple", "FOO=bar", "FOO", "bar", true},
		{"export prefix", "export FOO=bar", "FOO", "bar", true},
		{"spaces around =", "FOO = bar ", "FOO", "bar", true},
		{"empty value", "FOO=", "FOO", "", true},
		{"double quoted keeps spaces", `FOO=" bar baz "`, "FOO", " bar baz ", true},
		{"single quoted", `FOO='bar baz'`, "FOO", "bar baz", true},
		{"inline comment stripped", "FOO=bar # a note", "FOO", "bar", true},
		{"hash without space is kept", "PASS=se#cret", "PASS", "se#cret", true},
		{"quoted hash is kept", `PASS="se # cret"`, "PASS", "se # cret", true},
		{"url with fragment", "URL=http://h/p#frag", "URL", "http://h/p#frag", true},
		{"value containing =", "DSN=k=v;j=w", "DSN", "k=v;j=w", true},
		{"comment line", "# FOO=bar", "", "", false},
		{"blank line", "   ", "", "", false},
		{"no equals", "FOO", "", "", false},
		{"empty key", "=bar", "", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			k, v, ok := parseEnvLine(tc.line)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if k != tc.wantKey || v != tc.wantV {
				t.Errorf("got (%q, %q), want (%q, %q)", k, v, tc.wantKey, tc.wantV)
			}
		})
	}
}

// The precedence contract: real environment > .env.local > .env.
func TestLoadDotenvPrecedence(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, ".env"), "HF_BASE=from-env\nHF_OVERRIDDEN=from-env\nHF_REAL=from-env\n")
	write(t, filepath.Join(dir, ".env.local"), "HF_OVERRIDDEN=from-env-local\nHF_REAL=from-env-local\n")

	t.Setenv("HF_REAL", "from-real-environment")
	// Ensure the other two start unset, and clean up after the test.
	for _, k := range []string{"HF_BASE", "HF_OVERRIDDEN"} {
		os.Unsetenv(k)
		t.Cleanup(func() { os.Unsetenv(k) })
	}

	LoadDotenv(dir)

	want := map[string]string{
		"HF_BASE":       "from-env",             // only .env defines it
		"HF_OVERRIDDEN": "from-env-local",       // .env.local wins over .env
		"HF_REAL":       "from-real-environment", // the process env beats both
	}
	for k, w := range want {
		if got := os.Getenv(k); got != w {
			t.Errorf("%s = %q, want %q", k, got, w)
		}
	}
}

func TestLoadDotenvMissingFilesAreFine(t *testing.T) {
	LoadDotenv(t.TempDir()) // must not panic
}

func TestNormalizeWhatsApp(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"+237 6 12 34 56 78", "237612345678"},
		{"237612345678", "237612345678"},
		{"(237) 612-345-678", "237612345678"},
		{"", ""},
	} {
		if got := normalizeWhatsApp(tc.in); got != tc.want {
			t.Errorf("normalizeWhatsApp(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
