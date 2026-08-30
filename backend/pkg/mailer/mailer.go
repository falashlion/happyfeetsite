// Package mailer sends transactional HTML email over SMTP.
//
// It speaks three dialects so the same code works in local dev and production:
//
//   - port 465        → implicit TLS (SMTPS), used by most managed providers
//   - STARTTLS        → upgraded plaintext, used by Gmail/SES on port 587
//   - plain, no auth  → local capture servers such as Mailpit on port 1025
//
// Sending is fire-and-forget from the caller's perspective: Send blocks, but
// callers that must not be delayed by a slow relay should use SendAsync.
package mailer

import (
	"crypto/tls"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/smtp"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/happyfeet/api/pkg/config"
)

// ErrNotConfigured is returned when no SMTP host is set. It is not a failure
// worth alerting on — it just means email delivery is switched off.
var ErrNotConfigured = errors.New("mailer: SMTP not configured")

type Message struct {
	To      []string
	Cc      []string
	ReplyTo string
	Subject string
	HTML    string
	Text    string
}

type Mailer struct {
	cfg config.SMTPConfig
	log zerolog.Logger
	// dialTimeout bounds the whole SMTP conversation.
	dialTimeout time.Duration
}

func New(cfg config.SMTPConfig, log zerolog.Logger) *Mailer {
	return &Mailer{cfg: cfg, log: log.With().Str("component", "mailer").Logger(), dialTimeout: 20 * time.Second}
}

// Enabled reports whether a relay is configured.
func (m *Mailer) Enabled() bool { return m.cfg.Host != "" }

// SendAsync delivers in the background and logs the outcome. Use it on request
// paths — order placement must not fail or stall because a relay is slow.
func (m *Mailer) SendAsync(msg Message) {
	go func() {
		if err := m.Send(msg); err != nil && !errors.Is(err, ErrNotConfigured) {
			m.log.Error().Err(err).
				Strs("to", msg.To).
				Str("subject", msg.Subject).
				Msg("email delivery failed")
			return
		}
		m.log.Info().Strs("to", msg.To).Str("subject", msg.Subject).Msg("email sent")
	}()
}

func (m *Mailer) Send(msg Message) error {
	if !m.Enabled() {
		return ErrNotConfigured
	}
	recipients := append(append([]string{}, msg.To...), msg.Cc...)
	if len(recipients) == 0 {
		return errors.New("mailer: no recipients")
	}

	raw, err := m.build(msg)
	if err != nil {
		return err
	}

	addr := net.JoinHostPort(m.cfg.Host, fmt.Sprint(m.cfg.Port))
	client, err := m.dial(addr)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	if err := m.startTLS(client); err != nil {
		return err
	}
	if err := m.authenticate(client); err != nil {
		return err
	}

	if err := client.Mail(m.cfg.From); err != nil {
		return fmt.Errorf("mailer: MAIL FROM: %w", err)
	}
	for _, rcpt := range recipients {
		if err := client.Rcpt(rcpt); err != nil {
			return fmt.Errorf("mailer: RCPT TO %s: %w", rcpt, err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("mailer: DATA: %w", err)
	}
	if _, err := w.Write(raw); err != nil {
		_ = w.Close()
		return fmt.Errorf("mailer: write body: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("mailer: close body: %w", err)
	}
	return client.Quit()
}

// dial opens the connection, using implicit TLS on the SMTPS port.
func (m *Mailer) dial(addr string) (*smtp.Client, error) {
	if m.cfg.Port == 465 {
		conn, err := tls.DialWithDialer(
			&net.Dialer{Timeout: m.dialTimeout}, "tcp", addr,
			&tls.Config{ServerName: m.cfg.Host, MinVersion: tls.VersionTLS12},
		)
		if err != nil {
			return nil, fmt.Errorf("mailer: tls dial %s: %w", addr, err)
		}
		client, err := smtp.NewClient(conn, m.cfg.Host)
		if err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("mailer: smtp handshake: %w", err)
		}
		return client, nil
	}

	conn, err := net.DialTimeout("tcp", addr, m.dialTimeout)
	if err != nil {
		return nil, fmt.Errorf("mailer: dial %s: %w", addr, err)
	}
	_ = conn.SetDeadline(time.Now().Add(m.dialTimeout))
	client, err := smtp.NewClient(conn, m.cfg.Host)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("mailer: smtp handshake: %w", err)
	}
	return client, nil
}

// startTLS upgrades the connection when the server advertises STARTTLS. Local
// capture servers do not, and that is fine.
func (m *Mailer) startTLS(client *smtp.Client) error {
	if m.cfg.Port == 465 {
		return nil // already wrapped
	}
	if ok, _ := client.Extension("STARTTLS"); !ok {
		return nil
	}
	cfg := &tls.Config{ServerName: m.cfg.Host, MinVersion: tls.VersionTLS12}
	if err := client.StartTLS(cfg); err != nil {
		return fmt.Errorf("mailer: STARTTLS: %w", err)
	}
	return nil
}

func (m *Mailer) authenticate(client *smtp.Client) error {
	if m.cfg.Username == "" {
		return nil // unauthenticated relay (local dev)
	}
	if ok, _ := client.Extension("AUTH"); !ok {
		return nil
	}
	auth := smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("mailer: auth: %w", err)
	}
	return nil
}

// build assembles a multipart/alternative message so clients without HTML
// rendering (and spam filters that penalise HTML-only mail) still get content.
func (m *Mailer) build(msg Message) ([]byte, error) {
	boundary := fmt.Sprintf("hf_%d_boundary", time.Now().UnixNano())

	var b strings.Builder
	fmt.Fprintf(&b, "From: %s <%s>\r\n", mime.QEncoding.Encode("utf-8", m.cfg.FromName), m.cfg.From)
	fmt.Fprintf(&b, "To: %s\r\n", strings.Join(msg.To, ", "))
	if len(msg.Cc) > 0 {
		fmt.Fprintf(&b, "Cc: %s\r\n", strings.Join(msg.Cc, ", "))
	}
	if msg.ReplyTo != "" {
		fmt.Fprintf(&b, "Reply-To: %s\r\n", msg.ReplyTo)
	}
	fmt.Fprintf(&b, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", msg.Subject))
	fmt.Fprintf(&b, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	fmt.Fprintf(&b, "Message-ID: <%d.%s>\r\n", time.Now().UnixNano(), m.cfg.From)
	b.WriteString("MIME-Version: 1.0\r\n")
	fmt.Fprintf(&b, "Content-Type: multipart/alternative; boundary=%q\r\n\r\n", boundary)

	text := msg.Text
	if text == "" {
		text = "This message requires an HTML-capable email client."
	}
	fmt.Fprintf(&b, "--%s\r\n", boundary)
	b.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	b.WriteString(normalizeNewlines(text))
	b.WriteString("\r\n\r\n")

	fmt.Fprintf(&b, "--%s\r\n", boundary)
	b.WriteString("Content-Type: text/html; charset=\"utf-8\"\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	b.WriteString(normalizeNewlines(msg.HTML))
	b.WriteString("\r\n\r\n")

	fmt.Fprintf(&b, "--%s--\r\n", boundary)
	return []byte(b.String()), nil
}

// normalizeNewlines converts to CRLF, which SMTP requires, without doubling
// any CRLF that is already present.
func normalizeNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\n", "\r\n")
}
