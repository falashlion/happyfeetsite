package mailer

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"

	"github.com/rs/zerolog"

	"github.com/happyfeet/api/pkg/config"
)

// fakeRelay is a minimal SMTP server: enough of the conversation for the
// mailer to complete a delivery, and a switch to make it refuse one.
type fakeRelay struct {
	addr   string
	reject bool

	mu       sync.Mutex
	accepted []string // recipients of messages it took
	ln       net.Listener
}

func newRelay(t *testing.T, reject bool) *fakeRelay {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	r := &fakeRelay{addr: ln.Addr().String(), reject: reject, ln: ln}
	go r.serve()
	t.Cleanup(func() { _ = ln.Close() })
	return r
}

func (r *fakeRelay) port(t *testing.T) int {
	t.Helper()
	_, p, err := net.SplitHostPort(r.addr)
	if err != nil {
		t.Fatalf("split %q: %v", r.addr, err)
	}
	var n int
	if _, err := fmt.Sscanf(p, "%d", &n); err != nil {
		t.Fatalf("port %q: %v", p, err)
	}
	return n
}

func (r *fakeRelay) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.accepted)
}

func (r *fakeRelay) serve() {
	for {
		conn, err := r.ln.Accept()
		if err != nil {
			return
		}
		go r.handle(conn)
	}
}

func (r *fakeRelay) handle(conn net.Conn) {
	defer conn.Close()
	br := bufio.NewReader(conn)
	w := func(s string) { fmt.Fprintf(conn, "%s\r\n", s) }

	w("220 fake ESMTP")
	var rcpt string
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return
		}
		cmd := strings.ToUpper(strings.TrimSpace(line))
		switch {
		case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
			// No STARTTLS and no AUTH advertised: the mailer must then skip both,
			// which is exactly the local-capture path.
			w("250 fake")
		case strings.HasPrefix(cmd, "MAIL FROM"):
			if r.reject {
				w("550 sender refused")
				continue
			}
			w("250 OK")
		case strings.HasPrefix(cmd, "RCPT TO"):
			rcpt = strings.TrimSpace(line)
			w("250 OK")
		case strings.HasPrefix(cmd, "DATA"):
			w("354 go ahead")
			for {
				l, err := br.ReadString('\n')
				if err != nil {
					return
				}
				if strings.TrimRight(l, "\r\n") == "." {
					break
				}
			}
			r.mu.Lock()
			r.accepted = append(r.accepted, rcpt)
			r.mu.Unlock()
			w("250 queued")
		case strings.HasPrefix(cmd, "QUIT"):
			w("221 bye")
			return
		default:
			w("250 OK")
		}
	}
}

func relayCfg(t *testing.T, name string, r *fakeRelay) config.SMTPConfig {
	t.Helper()
	return config.SMTPConfig{
		Name: name, Host: "127.0.0.1", Port: r.port(t),
		From: name + "@example.com", FromName: "Happy Feet",
	}
}

func testMessage() Message {
	return Message{To: []string{"buyer@example.com"}, Subject: "Order HF-1", HTML: "<p>hi</p>"}
}

// Rotation is what actually raises the sending ceiling: with two free tiers
// configured, consecutive sends must land on different relays.
func TestRotateSpreadsAcrossRelays(t *testing.T) {
	a, b := newRelay(t, false), newRelay(t, false)
	m := New(config.MailConfig{
		Strategy:  "rotate",
		Providers: []config.SMTPConfig{relayCfg(t, "resend", a), relayCfg(t, "brevo", b)},
	}, zerolog.Nop())

	for i := 0; i < 4; i++ {
		if err := m.Send(testMessage()); err != nil {
			t.Fatalf("send %d: %v", i, err)
		}
	}
	if a.count() != 2 || b.count() != 2 {
		t.Errorf("rotation uneven: resend=%d brevo=%d, want 2 and 2", a.count(), b.count())
	}
}

// Failover keeps everything on the primary until it stops working.
func TestFailoverPrefersThePrimary(t *testing.T) {
	a, b := newRelay(t, false), newRelay(t, false)
	m := New(config.MailConfig{
		Strategy:  StrategyFailover,
		Providers: []config.SMTPConfig{relayCfg(t, "resend", a), relayCfg(t, "brevo", b)},
	}, zerolog.Nop())

	for i := 0; i < 3; i++ {
		if err := m.Send(testMessage()); err != nil {
			t.Fatalf("send %d: %v", i, err)
		}
	}
	if a.count() != 3 || b.count() != 0 {
		t.Errorf("resend=%d brevo=%d, want 3 and 0", a.count(), b.count())
	}
}

// A refusing relay must not lose the message — the next one takes it.
func TestARefusedMessageMovesToTheNextRelay(t *testing.T) {
	broken, good := newRelay(t, true), newRelay(t, false)
	m := New(config.MailConfig{
		Strategy:  StrategyFailover,
		Providers: []config.SMTPConfig{relayCfg(t, "broken", broken), relayCfg(t, "good", good)},
	}, zerolog.Nop())

	if err := m.Send(testMessage()); err != nil {
		t.Fatalf("send should have fallen through to the healthy relay: %v", err)
	}
	if good.count() != 1 {
		t.Errorf("healthy relay took %d messages, want 1", good.count())
	}
}

func TestEveryRelayFailingIsAnError(t *testing.T) {
	m := New(config.MailConfig{
		Providers: []config.SMTPConfig{
			relayCfg(t, "a", newRelay(t, true)),
			relayCfg(t, "b", newRelay(t, true)),
		},
	}, zerolog.Nop())

	err := m.Send(testMessage())
	if err == nil {
		t.Fatal("want an error when no relay accepts")
	}
	for _, name := range []string{"a", "b"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error should name relay %q: %v", name, err)
		}
	}
}

// A spent relay is skipped while another has allowance left.
func TestASpentRelayIsSkipped(t *testing.T) {
	capped, spare := newRelay(t, false), newRelay(t, false)
	cappedCfg := relayCfg(t, "capped", capped)
	cappedCfg.DailyLimit = 1

	m := New(config.MailConfig{
		Strategy:  StrategyFailover,
		Providers: []config.SMTPConfig{cappedCfg, relayCfg(t, "spare", spare)},
	}, zerolog.Nop())

	for i := 0; i < 3; i++ {
		if err := m.Send(testMessage()); err != nil {
			t.Fatalf("send %d: %v", i, err)
		}
	}
	if capped.count() != 1 {
		t.Errorf("capped relay took %d, want 1 (its daily limit)", capped.count())
	}
	if spare.count() != 2 {
		t.Errorf("spare relay took %d, want 2", spare.count())
	}
}

// When every relay looks spent the message is still attempted: the counters are
// an estimate, and dropping an order alert is worse than overshooting a quota.
func TestAllSpentStillAttempts(t *testing.T) {
	only := newRelay(t, false)
	cfg := relayCfg(t, "only", only)
	cfg.DailyLimit = 1

	m := New(config.MailConfig{Providers: []config.SMTPConfig{cfg}}, zerolog.Nop())
	for i := 0; i < 2; i++ {
		if err := m.Send(testMessage()); err != nil {
			t.Fatalf("send %d: %v", i, err)
		}
	}
	if only.count() != 2 {
		t.Errorf("relay took %d messages, want 2 — the over-limit send must still be attempted", only.count())
	}
}

// Each relay signs with its own verified sender, or the provider rejects it.
func TestEachRelayUsesItsOwnFromAddress(t *testing.T) {
	a, b := newRelay(t, false), newRelay(t, false)
	ca, cb := relayCfg(t, "resend", a), relayCfg(t, "brevo", b)
	m := New(config.MailConfig{Providers: []config.SMTPConfig{ca, cb}}, zerolog.Nop())

	for _, p := range m.providers {
		raw, err := m.build(p.cfg, testMessage())
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		want := "From: Happy Feet <" + p.cfg.From + ">"
		if !strings.Contains(string(raw), want) {
			t.Errorf("%s: header should carry its own sender %q\n%s", p.cfg.Name, p.cfg.From, firstLine(string(raw)))
		}
	}
}

func firstLine(s string) string {
	if i := strings.Index(s, "\r\n"); i > 0 {
		return s[:i]
	}
	return s
}

func TestNoRelayConfigured(t *testing.T) {
	m := New(config.MailConfig{Providers: []config.SMTPConfig{{Host: ""}}}, zerolog.Nop())
	if m.Enabled() {
		t.Fatal("a hostless relay must not count as configured")
	}
	if err := m.Send(testMessage()); err != ErrNotConfigured {
		t.Errorf("Send = %v, want ErrNotConfigured", err)
	}
}
