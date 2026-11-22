package alert

import (
	"fmt"
	"net/smtp"
	"strings"
	"sync"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

// EmailSink delivers detections as an email digest over SMTP. Rather than one
// message per event (which would bury an inbox), it buffers qualifying events
// and sends a single digest once the buffer reaches BatchSize — or when Flush
// is called (e.g. on shutdown). It is fail-soft like the other push sinks and
// honors a minimum-severity floor.
//
// The transport is plain net/smtp. Auth is optional: when username is empty the
// sink connects without authenticating (the common case for an internal relay),
// which is also what makes it testable against a mock listener with no TLS.
type EmailSink struct {
	addr   string // host:port of the SMTP server
	from   string
	to     []string
	auth   smtp.Auth
	minSev event.Severity
	batch  int

	// send is the transport, injectable for tests; defaults to smtp.SendMail.
	send func(addr string, a smtp.Auth, from string, to []string, msg []byte) error

	mu  sync.Mutex
	buf []event.Event
}

// EmailOptions configures an EmailSink.
type EmailOptions struct {
	Host      string
	Port      int
	From      string
	To        []string
	Username  string // empty = no auth
	Password  string
	MinSev    event.Severity
	BatchSize int // flush after this many buffered events (<=1 => send each immediately)
}

// NewEmail builds an EmailSink. Host, From and at least one To are required.
func NewEmail(opts EmailOptions) (*EmailSink, error) {
	if opts.Host == "" {
		return nil, fmt.Errorf("email: host is required")
	}
	if opts.From == "" {
		return nil, fmt.Errorf("email: from is required")
	}
	if len(opts.To) == 0 {
		return nil, fmt.Errorf("email: at least one recipient is required")
	}
	port := opts.Port
	if port == 0 {
		port = 25
	}
	batch := opts.BatchSize
	if batch < 1 {
		batch = 1
	}
	var auth smtp.Auth
	if opts.Username != "" {
		auth = smtp.PlainAuth("", opts.Username, opts.Password, opts.Host)
	}
	return &EmailSink{
		addr:   fmt.Sprintf("%s:%d", opts.Host, port),
		from:   opts.From,
		to:     append([]string(nil), opts.To...),
		auth:   auth,
		minSev: opts.MinSev,
		batch:  batch,
		send:   smtp.SendMail,
	}, nil
}

// Emit buffers the event (if it meets the floor) and flushes a digest once the
// buffer reaches BatchSize.
func (s *EmailSink) Emit(e event.Event) error {
	if e.Severity < s.minSev {
		return nil
	}
	s.mu.Lock()
	s.buf = append(s.buf, e)
	full := len(s.buf) >= s.batch
	s.mu.Unlock()
	if full {
		return s.Flush()
	}
	return nil
}

// Flush sends the buffered events as one digest and clears the buffer. It is a
// no-op when the buffer is empty. Safe to call on shutdown.
func (s *EmailSink) Flush() error {
	s.mu.Lock()
	if len(s.buf) == 0 {
		s.mu.Unlock()
		return nil
	}
	batch := s.buf
	s.buf = nil
	s.mu.Unlock()

	msg := s.render(batch)
	if err := s.send(s.addr, s.auth, s.from, s.to, msg); err != nil {
		return fmt.Errorf("email: send digest: %w", err)
	}
	return nil
}

// render builds the RFC 5322 message (headers + digest body) for a batch.
func (s *EmailSink) render(batch []event.Event) []byte {
	subject := fmt.Sprintf("Heramba: %d detection(s)", len(batch))
	if len(batch) == 1 {
		subject = fmt.Sprintf("Heramba: %s from %s", batch[0].Kind, orDash(batch[0].Source))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", s.from)
	fmt.Fprintf(&b, "To: %s\r\n", strings.Join(s.to, ", "))
	fmt.Fprintf(&b, "Subject: %s\r\n", subject)
	fmt.Fprintf(&b, "Date: %s\r\n", time.Now().UTC().Format(time.RFC1123Z))
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	b.WriteString("\r\n")
	fmt.Fprintf(&b, "Heramba detection digest — %d event(s)\r\n\r\n", len(batch))
	for _, e := range batch {
		fmt.Fprintf(&b, "[%s] %s %s\r\n", e.SeverityS, e.Kind, e.Time.UTC().Format(time.RFC3339))
		fmt.Fprintf(&b, "    source=%s target=%s confidence=%.2f\r\n", orDash(e.Source), orDash(e.Target), e.Confidence)
		if e.Detail != "" {
			fmt.Fprintf(&b, "    detail: %s\r\n", e.Detail)
		}
		if c := e.Labels["campaign"]; c != "" {
			fmt.Fprintf(&b, "    campaign: %s\r\n", c)
		}
		b.WriteString("\r\n")
	}
	return []byte(b.String())
}
