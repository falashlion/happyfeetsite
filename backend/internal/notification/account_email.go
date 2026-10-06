package notification

import (
	"fmt"
	"strings"

	"github.com/happyfeet/api/pkg/mailer"
)

// AccountEmail is the data behind every non-order message: welcome, password
// reset, sign-in code. One shape and one template, because these differ only in
// wording — a separate template per message would be four copies of the same
// markup drifting apart.
type AccountEmail struct {
	To   string
	Name string

	Intro string
	Outro string

	// Code, when set, renders the one-time code panel.
	Code      string
	CodeLabel string
	ExpiresIn string

	// ActionURL, when set, renders the primary button.
	ActionURL   string
	ActionLabel string

	WhatsAppLink string
	SupportEmail string
}

// SendWelcome greets a new account. Fire-and-forget: registration must not fail
// or stall because a mail relay is slow.
func (s *EmailService) SendWelcome(to, firstName string) {
	if !s.canSend(to, "welcome") {
		return
	}

	a := AccountEmail{
		To:   to,
		Name: firstName,
		Intro: fmt.Sprintf(
			"Welcome to %s. Your account is ready — your bag, your wishlist and your order history now follow you between devices.",
			s.storeName()),
		ActionURL:   s.site,
		ActionLabel: "Start browsing",
		Outro:       "You can change your details any time from your account page.",
	}
	s.sendAccount(a, templateData{
		Subject:   fmt.Sprintf("Welcome to %s", s.storeName()),
		Preheader: "Your account is ready.",
		Eyebrow:   "Welcome",
		Headline:  greet(firstName),
		Subhead:   "Everything in one place, on every device.",
	})
}

// SendPasswordResetCode delivers a reset code. The caller has already decided
// an account exists; this never reveals that either way on its own.
func (s *EmailService) SendPasswordResetCode(to, firstName, code, expiresIn string) {
	if !s.canSend(to, "password reset") {
		return
	}

	a := AccountEmail{
		To:        to,
		Name:      firstName,
		Intro:     "Use this code to choose a new password. If you did not ask for it, you can ignore this message — nothing has changed.",
		Code:      code,
		CodeLabel: "Reset code",
		ExpiresIn: expiresIn,
		Outro:     "For your security, never share this code with anyone — including someone claiming to be from our team.",
	}
	s.sendAccount(a, templateData{
		Subject:   "Your password reset code",
		Preheader: fmt.Sprintf("Code %s — expires in %s.", code, expiresIn),
		Eyebrow:   "Password reset",
		Headline:  "Choose a new password",
	})
}

// SendVerificationCode delivers a sign-in or address-verification code.
func (s *EmailService) SendVerificationCode(to, firstName, code, expiresIn string) {
	if !s.canSend(to, "verification") {
		return
	}

	a := AccountEmail{
		To:        to,
		Name:      firstName,
		Intro:     "Enter this code to confirm it is you.",
		Code:      code,
		CodeLabel: "Verification code",
		ExpiresIn: expiresIn,
		Outro:     "If you did not ask for this code, you can safely ignore this message.",
	}
	s.sendAccount(a, templateData{
		Subject:   "Your verification code",
		Preheader: fmt.Sprintf("Code %s — expires in %s.", code, expiresIn),
		Eyebrow:   "Verification",
		Headline:  "Confirm it is you",
	})
}

// canSend centralises the two reasons an account email is skipped, so each
// caller stays a straight line.
func (s *EmailService) canSend(to, kind string) bool {
	if !s.Enabled() {
		s.log.Debug().Str("kind", kind).Msg("email disabled; skipping")
		return false
	}
	if strings.TrimSpace(to) == "" {
		s.log.Debug().Str("kind", kind).Msg("no recipient address; skipping")
		return false
	}
	return true
}

func (s *EmailService) sendAccount(a AccountEmail, data templateData) {
	a.SupportEmail = s.store.SupportEmail
	if s.store.WhatsAppNumber != "" {
		a.WhatsAppLink = "https://wa.me/" + s.store.WhatsAppNumber
	}
	data.Account = a

	html, err := s.render(s.accountTmpl, data)
	if err != nil {
		s.log.Error().Err(err).Str("subject", data.Subject).Msg("render account email")
		return
	}

	s.mail.SendAsync(mailer.Message{
		To:      []string{a.To},
		Subject: data.Subject,
		HTML:    html,
		Text:    accountPlainText(data, a),
	})
}

func (s *EmailService) storeName() string {
	if s.store.Name != "" {
		return s.store.Name
	}
	return "Happy Feet"
}

func greet(firstName string) string {
	if n := strings.TrimSpace(firstName); n != "" {
		return "Hello " + n + "."
	}
	return "Hello."
}

// accountPlainText is the text/plain alternative. Every message carries one:
// a code-bearing email that renders as a blank page in a text-only client is
// a locked-out customer.
func accountPlainText(data templateData, a AccountEmail) string {
	var b strings.Builder
	b.WriteString(data.Headline + "\n\n")
	if a.Intro != "" {
		b.WriteString(a.Intro + "\n\n")
	}
	if a.Code != "" {
		b.WriteString(strings.ToUpper(a.CodeLabel) + ": " + a.Code + "\n")
		if a.ExpiresIn != "" {
			b.WriteString("Expires in " + a.ExpiresIn + ".\n")
		}
		b.WriteString("\n")
	}
	if a.ActionURL != "" {
		b.WriteString(a.ActionLabel + ": " + a.ActionURL + "\n\n")
	}
	if a.Outro != "" {
		b.WriteString(a.Outro + "\n\n")
	}
	if a.WhatsAppLink != "" {
		b.WriteString("Need a hand? WhatsApp " + a.WhatsAppLink + "\n")
	}
	if a.SupportEmail != "" {
		b.WriteString("Email " + a.SupportEmail + "\n")
	}
	return b.String()
}
