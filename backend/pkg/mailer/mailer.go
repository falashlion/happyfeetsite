// Package mailer sends transactional HTML email over SMTP, across one or more
// relays.
//
// It speaks three dialects so the same code works in local dev and production:
//
//   - port 465        → implicit TLS (SMTPS), used by most managed providers
//   - STARTTLS        → upgraded plaintext, used by Gmail/SES on port 587
//   - plain, no auth  → local capture servers such as Mailpit on port 1025
//
// # Several relays at once
//
// A Mailer holds an ordered pool of providers. Two free tiers side by side
// (Resend 3,000/month, Brevo 300/day) send more than either alone, and one
// provider having a bad day stops being the store's bad day:
//
//   - "rotate" (default) round-robins, so volume is spread and both allowances
//     are actually used;
//   - "failover" always prefers the first and reaches for the rest only when it
//     fails or is spent.
//
// Either way a send that a relay rejects is retried on the next relay in line
// before it is reported as failed.
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
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"

	"github.com/happyfeet/api/pkg/config"
)

// ErrNotConfigured is returned when no SMTP host is set. It is not a failure
// worth alerting on — it just means email delivery is switched off.
var ErrNotConfigured = errors.New("mailer: SMTP not configured")

// StrategyFailover keeps every message on the first relay until it stops
// working; anything else round-robins.
const StrategyFailover = "failover"

type Message struct {
	To      []string
	Cc      []string
	ReplyTo string
	Subject string
	HTML    string
	Text    string
}

// provider is one relay plus the running count of what has been sent through
// it, used to stay inside a free tier.
type provider struct {
	cfg config.SMTPConfig

	mu         sync.Mutex
	dayStamp   string // YYYY-MM-DD, UTC
	dayCount   int
	monthStamp string // YYYY-MM, UTC
	monthCount int
}

// spent reports whether this relay has used up an allowance we were told about.
//
// The counters are in-memory, so a restart forgets them. That is deliberate and
// safe: they only decide which relay is *tried first*. A relay that is actually
// over quota rejects the message and delivery falls through to the next one.
func (p *provider) spent(now time.Time) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.rollLocked(now)
	if p.cfg.DailyLimit > 0 && p.dayCount >= p.cfg.DailyLimit {
		return true
	}
	return p.cfg.MonthlyLimit > 0 && p.monthCount >= p.cfg.MonthlyLimit
}

func (p *provider) record(now time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.rollLocked(now)
	p.dayCount++
	p.monthCount++
}

// rollLocked resets the counters when their window has turned over. Callers
// hold p.mu.
func (p *provider) rollLocked(now time.Time) {
	if day := now.UTC().Format("2006-01-02"); day != p.dayStamp {
		p.dayStamp, p.dayCount = day, 0
	}
	if month := now.UTC().Format("2006-01"); month != p.monthStamp {
		p.monthStamp, p.monthCount = month, 0
	}
}

// Usage is a point-in-time view of one relay, for logs and diagnostics.
type Usage struct {
	Name         string
	Host         string
	Port         int
	From         string
	SentToday    int
	SentMonth    int
	DailyLimit   int
	MonthlyLimit int
}

type Mailer struct {
	providers []*provider
	strategy  string
	// cursor advances once per send under the rotate strategy.
	cursor atomic.Uint64
	log    zerolog.Logger
	// dialTimeout bounds the whole SMTP conversation.
	dialTimeout time.Duration
}

// New builds a mailer over every relay given, in preference order. Entries
// without a host are dropped, so an unconfigured second provider costs nothing.
func New(cfg config.MailConfig, log zerolog.Logger) *Mailer {
	m := &Mailer{
		strategy:    strings.ToLower(strings.TrimSpace(cfg.Strategy)),
		log:         log.With().Str("component", "mailer").Logger(),
		dialTimeout: 20 * time.Second,
	}
	for _, c := range cfg.Providers {
		if strings.TrimSpace(c.Host) == "" {
			continue
		}
		if c.Name == "" {
			c.Name = c.Host
		}
		m.providers = append(m.providers, &provider{cfg: c})
	}
	return m
}

// NewSingle is the one-relay convenience used by tests and by any caller that
// holds a bare SMTPConfig.
func NewSingle(cfg config.SMTPConfig, log zerolog.Logger) *Mailer {
	return New(config.MailConfig{Providers: []config.SMTPConfig{cfg}}, log)
}

// Enabled reports whether at least one relay is configured.
func (m *Mailer) Enabled() bool { return len(m.providers) > 0 }

// Providers lists the configured relays and what each has sent, oldest window
// first. Safe to call at any time.
func (m *Mailer) Providers() []Usage {
	now := time.Now()
	out := make([]Usage, 0, len(m.providers))
	for _, p := range m.providers {
		p.mu.Lock()
		p.rollLocked(now)
		out = append(out, Usage{
			Name: p.cfg.Name, Host: p.cfg.Host, Port: p.cfg.Port, From: p.cfg.From,
			SentToday: p.dayCount, SentMonth: p.monthCount,
			DailyLimit: p.cfg.DailyLimit, MonthlyLimit: p.cfg.MonthlyLimit,
		})
		p.mu.Unlock()
	}
	return out
}

// SendAsync delivers in the background and logs the outcome. Use it on request
// paths — order placement must not fail or stall because a relay is slow.
func (m *Mailer) SendAsync(msg Message) {
	go func() {
		via, err := m.send(msg)
		if err != nil {
			if !errors.Is(err, ErrNotConfigured) {
				m.log.Error().Err(err).
					Strs("to", msg.To).
					Str("subject", msg.Subject).
					Msg("email delivery failed on every relay")
			}
			return
		}
		m.log.Info().Strs("to", msg.To).Str("subject", msg.Subject).Str("via", via).Msg("email sent")
	}()
}

// Send delivers the message, trying each relay in turn. It returns nil as soon
// as one accepts it.
func (m *Mailer) Send(msg Message) error {
	_, err := m.send(msg)
	return err
}

// SendVia delivers through one named relay only, without falling through to
// the others. It exists for the -mailtest probe, which has to prove that each
// provider works on its own rather than that *some* provider works.
func (m *Mailer) SendVia(name string, msg Message) error {
	for _, p := range m.providers {
		if !strings.EqualFold(p.cfg.Name, name) {
			continue
		}
		if err := m.deliver(p, msg); err != nil {
			return err
		}
		p.record(time.Now())
		return nil
	}
	return fmt.Errorf("mailer: no relay named %q", name)
}

// send walks the pool and returns the name of the relay that accepted.
func (m *Mailer) send(msg Message) (string, error) {
	if !m.Enabled() {
		return "", ErrNotConfigured
	}
	if len(msg.To)+len(msg.Cc) == 0 {
		return "", errors.New("mailer: no recipients")
	}

	order := m.order()
	now := time.Now()
	var errs []error

	// First pass: only relays believed to have allowance left.
	var skipped []*provider
	for _, p := range order {
		if p.spent(now) {
			skipped = append(skipped, p)
			continue
		}
		if err := m.deliver(p, msg); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", p.cfg.Name, err))
			m.log.Warn().Err(err).Str("relay", p.cfg.Name).Msg("relay refused the message; trying the next")
			continue
		}
		p.record(now)
		return p.cfg.Name, nil
	}

	// Second pass: every relay looks spent. Our counters are only an estimate —
	// they reset on restart and know nothing about mail sent from elsewhere — so
	// an order alert is worth one real attempt before it is given up on.
	for _, p := range skipped {
		m.log.Warn().Str("relay", p.cfg.Name).Msg("all relays are at their configured limit; attempting anyway")
		if err := m.deliver(p, msg); err != nil {
			errs = append(errs, fmt.Errorf("%s (over limit): %w", p.cfg.Name, err))
			continue
		}
		p.record(now)
		return p.cfg.Name, nil
	}

	return "", fmt.Errorf("mailer: every relay failed: %w", errors.Join(errs...))
}

// order returns the providers in the sequence to try them.
func (m *Mailer) order() []*provider {
	out := make([]*provider, len(m.providers))
	if m.strategy == StrategyFailover || len(m.providers) == 1 {
		copy(out, m.providers)
		return out
	}
	// Round-robin: start one further along each send, keeping the rest of the
	// list as the fallback chain.
	start := int(m.cursor.Add(1)-1) % len(m.providers)
	for i := range m.providers {
		out[i] = m.providers[(start+i)%len(m.providers)]
	}
	return out
}

// deliver runs one full SMTP conversation against a single relay.
func (m *Mailer) deliver(p *provider, msg Message) error {
	recipients := append(append([]string{}, msg.To...), msg.Cc...)

	raw, err := m.build(p.cfg, msg)
	if err != nil {
		return err
	}

	addr := net.JoinHostPort(p.cfg.Host, fmt.Sprint(p.cfg.Port))
	client, err := m.dial(p.cfg, addr)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	if err := m.startTLS(p.cfg, client); err != nil {
		return err
	}
	if err := m.authenticate(p.cfg, client); err != nil {
		return err
	}

	if err := client.Mail(p.cfg.From); err != nil {
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
func (m *Mailer) dial(cfg config.SMTPConfig, addr string) (*smtp.Client, error) {
	if cfg.Port == 465 {
		conn, err := tls.DialWithDialer(
			&net.Dialer{Timeout: m.dialTimeout}, "tcp", addr,
			&tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12},
		)
		if err != nil {
			return nil, fmt.Errorf("mailer: tls dial %s: %w", addr, err)
		}
		client, err := smtp.NewClient(conn, cfg.Host)
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
	client, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("mailer: smtp handshake: %w", err)
	}
	return client, nil
}

// startTLS upgrades the connection when the server advertises STARTTLS. Local
// capture servers do not, and that is fine.
func (m *Mailer) startTLS(cfg config.SMTPConfig, client *smtp.Client) error {
	if cfg.Port == 465 {
		return nil // already wrapped
	}
	if ok, _ := client.Extension("STARTTLS"); !ok {
		return nil
	}
	tlsCfg := &tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12}
	if err := client.StartTLS(tlsCfg); err != nil {
		return fmt.Errorf("mailer: STARTTLS: %w", err)
	}
	return nil
}

func (m *Mailer) authenticate(cfg config.SMTPConfig, client *smtp.Client) error {
	if cfg.Username == "" {
		return nil // unauthenticated relay (local dev)
	}
	if ok, _ := client.Extension("AUTH"); !ok {
		return nil
	}
	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("mailer: auth: %w", err)
	}
	return nil
}

// build assembles a multipart/alternative message so clients without HTML
// rendering (and spam filters that penalise HTML-only mail) still get content.
//
// The From header is the relay's own, because a provider will refuse — or
// silently damage the authentication of — a sender it has not verified.
func (m *Mailer) build(cfg config.SMTPConfig, msg Message) ([]byte, error) {
	boundary := fmt.Sprintf("hf_%d_boundary", time.Now().UnixNano())

	var b strings.Builder
	fmt.Fprintf(&b, "From: %s <%s>\r\n", mime.QEncoding.Encode("utf-8", cfg.FromName), cfg.From)
	fmt.Fprintf(&b, "To: %s\r\n", strings.Join(msg.To, ", "))
	if len(msg.Cc) > 0 {
		fmt.Fprintf(&b, "Cc: %s\r\n", strings.Join(msg.Cc, ", "))
	}
	if msg.ReplyTo != "" {
		fmt.Fprintf(&b, "Reply-To: %s\r\n", msg.ReplyTo)
	}
	fmt.Fprintf(&b, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", msg.Subject))
	fmt.Fprintf(&b, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	fmt.Fprintf(&b, "Message-ID: <%d.%s>\r\n", time.Now().UnixNano(), cfg.From)
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
