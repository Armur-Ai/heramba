package alert

import (
	"bufio"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

// mockSMTP is a tiny SMTP server good enough for net/smtp.SendMail without auth
// or TLS. It records the DATA body of the first message it receives.
type mockSMTP struct {
	host string
	port int
	ln   net.Listener
	mu   sync.Mutex
	data string
	done chan struct{}
}

func newMockSMTP(t *testing.T) *mockSMTP {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	host, portStr, _ := net.SplitHostPort(ln.Addr().String())
	port, _ := strconv.Atoi(portStr)
	m := &mockSMTP{host: host, port: port, ln: ln, done: make(chan struct{})}
	go m.serve()
	return m
}

func (m *mockSMTP) serve() {
	defer close(m.done)
	conn, err := m.ln.Accept()
	if err != nil {
		return
	}
	defer conn.Close()
	r := bufio.NewReader(conn)
	w := bufio.NewWriter(conn)
	write := func(s string) { _, _ = w.WriteString(s); _ = w.Flush() }

	write("220 mock ESMTP\r\n")
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		cmd := strings.ToUpper(strings.TrimSpace(line))
		switch {
		case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
			write("250-mock greets you\r\n250 HELP\r\n")
		case strings.HasPrefix(cmd, "MAIL FROM"):
			write("250 OK\r\n")
		case strings.HasPrefix(cmd, "RCPT TO"):
			write("250 OK\r\n")
		case cmd == "DATA":
			write("354 End data with <CR><LF>.<CR><LF>\r\n")
			var body strings.Builder
			for {
				dl, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if strings.TrimRight(dl, "\r\n") == "." {
					break
				}
				body.WriteString(dl)
			}
			m.mu.Lock()
			m.data = body.String()
			m.mu.Unlock()
			write("250 OK: queued\r\n")
		case cmd == "QUIT":
			write("221 Bye\r\n")
			return
		default:
			write("250 OK\r\n")
		}
	}
}

func (m *mockSMTP) body(t *testing.T) string {
	t.Helper()
	select {
	case <-m.done:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for SMTP delivery")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.data
}

func (m *mockSMTP) opts() EmailOptions {
	return EmailOptions{Host: m.host, Port: m.port, From: "heramba@example.com", To: []string{"soc@example.com"}}
}

func TestEmailSinkSendsDigestAtBatchSize(t *testing.T) {
	m := newMockSMTP(t)
	o := m.opts()
	o.BatchSize = 2
	o.MinSev = event.SeverityLow
	s, err := NewEmail(o)
	if err != nil {
		t.Fatal(err)
	}
	// First event buffers (no send yet), second triggers the digest.
	if err := s.Emit(event.Deception(event.KindDecoyHit, "10.0.0.1", "/.env", "probe")); err != nil {
		t.Fatal(err)
	}
	if err := s.Emit(event.Deception(event.KindCanaryTriggered, "10.0.0.1", "hrmb_abc", "token used").WithLabel("campaign", "sweep")); err != nil {
		t.Fatal(err)
	}

	body := m.body(t)
	for _, want := range []string{"Subject: Heramba: 2 detection(s)", "decoy_hit", "canary_triggered", "10.0.0.1", "campaign: sweep"} {
		if !strings.Contains(body, want) {
			t.Errorf("digest missing %q in:\n%s", want, body)
		}
	}
}

func TestEmailSinkFloorAndFlushBuffer(t *testing.T) {
	m := newMockSMTP(t)
	o := m.opts()
	o.MinSev = event.SeverityHigh
	o.BatchSize = 10 // won't hit batch; rely on Flush
	s, err := NewEmail(o)
	if err != nil {
		t.Fatal(err)
	}
	// Below floor: dropped, buffer stays empty.
	if err := s.Emit(event.New(event.KindProbe, event.SeverityLow, "x", "/x", "d")); err != nil {
		t.Fatal(err)
	}
	// At floor: buffered.
	if err := s.Emit(event.New(event.KindEgressBlocked, event.SeverityHigh, "y", "z", "blocked")); err != nil {
		t.Fatal(err)
	}
	// Explicit flush sends the single buffered high-sev event.
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	body := m.body(t)
	if !strings.Contains(body, "egress_blocked") {
		t.Errorf("flushed digest missing the high-sev event:\n%s", body)
	}
	if strings.Contains(body, "probe") {
		t.Errorf("below-floor event must not appear in digest:\n%s", body)
	}
}

func TestEmailFlushEmptyIsNoop(t *testing.T) {
	called := false
	s, err := NewEmail(EmailOptions{Host: "h", From: "a@b", To: []string{"c@d"}})
	if err != nil {
		t.Fatal(err)
	}
	s.send = func(string, smtp.Auth, string, []string, []byte) error { called = true; return nil }
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Error("Flush with empty buffer must not send")
	}
}

func TestNewEmailValidation(t *testing.T) {
	cases := []EmailOptions{
		{From: "a@b", To: []string{"c@d"}}, // no host
		{Host: "h", To: []string{"c@d"}},   // no from
		{Host: "h", From: "a@b"},           // no recipients
	}
	for i, o := range cases {
		if _, err := NewEmail(o); err == nil {
			t.Errorf("case %d: expected validation error", i)
		}
	}
}

// flushCounter is a Flusher sink counting flush calls, for FlushAll.
type flushCounter struct{ n int }

func (f *flushCounter) Emit(event.Event) error { return nil }
func (f *flushCounter) Flush() error           { f.n++; return nil }

func TestFlushAllReachesMultiSinkMembers(t *testing.T) {
	fc := &flushCounter{}
	// Single sink.
	if err := FlushAll(fc); err != nil || fc.n != 1 {
		t.Fatalf("single flush: n=%d err=%v", fc.n, err)
	}
	// Inside a MultiSink alongside a non-flusher.
	fc2 := &flushCounter{}
	m := MultiSink{Discard, fc2, NewText(&strings.Builder{})}
	if err := FlushAll(m); err != nil {
		t.Fatal(err)
	}
	if fc2.n != 1 {
		t.Fatalf("multisink member flush n=%d, want 1", fc2.n)
	}
}
