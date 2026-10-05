package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad_Defaults(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.App.Name != "happyfeet-api" {
		t.Fatalf("expected happyfeet-api, got %s", cfg.App.Name)
	}
	if cfg.HTTP.Port != 8080 {
		t.Fatalf("expected 8080, got %d", cfg.HTTP.Port)
	}
	if cfg.JWT.AccessTokenExpiry != 15*time.Minute {
		t.Fatalf("expected 15m, got %v", cfg.JWT.AccessTokenExpiry)
	}
}

func TestLoad_EnvOverride(t *testing.T) {
	os.Setenv("HTTP_PORT", "9090")
	os.Setenv("APP_ENV", "production")
	defer func() {
		os.Unsetenv("HTTP_PORT")
		os.Unsetenv("APP_ENV")
	}()
	cfg, _ := Load()
	if cfg.HTTP.Port != 9090 {
		t.Fatalf("expected 9090, got %d", cfg.HTTP.Port)
	}
	if !cfg.IsProd() {
		t.Fatal("expected production env")
	}
}

func TestAddr(t *testing.T) {
	cfg, _ := Load()
	addr := cfg.Addr()
	if addr == "" {
		t.Fatal("expected non-empty addr")
	}
}

// A second relay is meant to be one block of environment variables — no code
// change, no restart ceremony beyond the usual one.
func TestLoad_SecondSMTPProvider(t *testing.T) {
	for k, v := range map[string]string{
		"SMTP_HOST": "smtp.resend.com", "SMTP_PORT": "587",
		"SMTP_USERNAME": "resend", "SMTP_PASSWORD": "re_xxx",
		"SMTP_FROM": "orders@happyfeet.cm", "SMTP_FROM_NAME": "Happy Feet",
		"SMTP_MONTHLY_LIMIT": "3000",
		"SMTP2_HOST":         "smtp-relay.brevo.com", "SMTP2_PORT": "587",
		"SMTP2_USERNAME": "9a1b2c@smtp-brevo.com", "SMTP2_PASSWORD": "xsmtpsib-yyy",
		"SMTP2_DAILY_LIMIT": "300",
	} {
		t.Setenv(k, v)
	}

	cfg, _ := Load()
	if len(cfg.Mail.Providers) != 2 {
		t.Fatalf("got %d providers, want 2", len(cfg.Mail.Providers))
	}

	primary, second := cfg.Mail.Providers[0], cfg.Mail.Providers[1]
	if primary.Name != "resend" || second.Name != "brevo" {
		t.Errorf("names = %q, %q — want them derived from the hosts", primary.Name, second.Name)
	}
	if primary.MonthlyLimit != 3000 || second.DailyLimit != 300 {
		t.Errorf("limits not read: monthly=%d daily=%d", primary.MonthlyLimit, second.DailyLimit)
	}
	// The sender identity carries over, so a second relay needs credentials only.
	if second.From != "orders@happyfeet.cm" || second.FromName != "Happy Feet" {
		t.Errorf("second relay should inherit the sender: %q <%q>", second.FromName, second.From)
	}
	if cfg.Mail.Strategy != "rotate" {
		t.Errorf("default strategy = %q, want rotate", cfg.Mail.Strategy)
	}
}

// With no SMTP2_HOST there is exactly one relay — the empty slot is not a
// half-configured provider that fails every send.
func TestLoad_SecondProviderAbsent(t *testing.T) {
	t.Setenv("SMTP_HOST", "smtp.resend.com")
	t.Setenv("SMTP2_HOST", "")
	cfg, _ := Load()
	if len(cfg.Mail.Providers) != 1 {
		t.Fatalf("got %d providers, want 1", len(cfg.Mail.Providers))
	}
}
