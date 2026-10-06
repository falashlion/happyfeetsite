package notification

import (
	"strings"
	"testing"

	"github.com/happyfeet/api/pkg/config"
	"github.com/happyfeet/api/pkg/mailer"
	"github.com/rs/zerolog"
)

// A reset email whose HTML renders but omits the code is worse than no email:
// the customer sees a message that looks right and cannot act on it. Render the
// real template and assert the code survives into both parts.
func TestAccountEmailRendersCodeInHTMLAndText(t *testing.T) {
	svc := newTestService(t)

	a := AccountEmail{
		To:        "customer@example.com",
		Name:      "Felix",
		Intro:     "Use this code to choose a new password.",
		Code:      "483920",
		CodeLabel: "Reset code",
		ExpiresIn: "5 minutes",
	}
	a.SupportEmail = svc.store.SupportEmail
	a.WhatsAppLink = "https://wa.me/" + svc.store.WhatsAppNumber

	data := templateData{
		Subject:  "Your password reset code",
		Eyebrow:  "Password reset",
		Headline: "Choose a new password",
		Account:  a,
	}

	html, err := svc.render(svc.accountTmpl, data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, "483920") {
		t.Error("HTML body must contain the code")
	}
	if !strings.Contains(html, "Choose a new password") {
		t.Error("HTML body must contain the headline from the layout")
	}
	if !strings.Contains(html, "wa.me/237612345678") {
		t.Error("HTML body must carry the store WhatsApp link")
	}

	text := accountPlainText(data, a)
	if !strings.Contains(text, "483920") {
		t.Error("plain-text alternative must contain the code")
	}
	if !strings.Contains(text, "5 minutes") {
		t.Error("plain-text alternative must state the expiry")
	}
}

// The welcome email has no code and a button instead; the same template must
// cope without rendering an empty code panel.
func TestWelcomeEmailRendersActionNotCode(t *testing.T) {
	svc := newTestService(t)

	a := AccountEmail{
		To:          "customer@example.com",
		Name:        "Felix",
		Intro:       "Welcome to Happy Feet.",
		ActionURL:   "http://localhost:3000",
		ActionLabel: "Start browsing",
	}
	data := templateData{Subject: "Welcome", Headline: greet("Felix"), Account: a}

	html, err := svc.render(svc.accountTmpl, data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, "Start browsing") {
		t.Error("welcome email must render its call to action")
	}
	if strings.Contains(html, "Reset code") {
		t.Error("welcome email must not render the code panel")
	}
	if !strings.Contains(html, "Hello Felix.") {
		t.Error("welcome email should greet by first name")
	}
}

func TestGreetFallsBackWithoutAName(t *testing.T) {
	if got := greet("  "); got != "Hello." {
		t.Errorf("greet(blank) = %q, want %q", got, "Hello.")
	}
	if got := greet("Felix"); got != "Hello Felix." {
		t.Errorf("greet = %q", got)
	}
}

// Every account send must be a no-op when email is unconfigured or the address
// is blank — these are called from request paths that must not fail.
func TestAccountSendsAreNoopWhenDisabled(t *testing.T) {
	cfg := &config.Config{}
	svc, err := NewEmailService(mailer.NewSingle(cfg.SMTP, zerolog.Nop()), cfg, zerolog.Nop())
	if err != nil {
		t.Fatal(err)
	}
	if svc.Enabled() {
		t.Fatal("expected a disabled service with no SMTP host")
	}

	// None of these may panic.
	svc.SendWelcome("a@example.com", "Felix")
	svc.SendPasswordResetCode("a@example.com", "Felix", "123456", "5 minutes")
	svc.SendVerificationCode("a@example.com", "Felix", "123456", "5 minutes")
}

func TestAccountSendSkipsBlankRecipient(t *testing.T) {
	svc := newTestService(t)
	if !svc.canSend("ok@example.com", "test") {
		t.Error("a configured service with a recipient should be able to send")
	}
	if svc.canSend("   ", "test") {
		t.Error("a blank recipient must be refused")
	}
}
