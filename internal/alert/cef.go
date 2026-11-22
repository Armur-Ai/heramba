package alert

import (
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

// CEFSink emits detections as ArcSight CEF records, the lingua franca most SIEMs
// (ArcSight, QRadar, LogRhythm, …) ingest. It can write to any io.Writer or,
// via NewSyslogCEF, to a UDP/TCP syslog collector. Fail-soft and severity-floored.
type CEFSink struct {
	mu            sync.Mutex
	w             io.Writer
	network, addr string // set for syslog transports, to allow reconnect
	host          string
	minSev        event.Severity
}

// NewCEFWriter writes CEF lines to an arbitrary writer (a file, stdout, a conn).
func NewCEFWriter(w io.Writer, minSeverity event.Severity) *CEFSink {
	host, _ := os.Hostname()
	if host == "" {
		host = "heramba"
	}
	return &CEFSink{w: w, host: host, minSev: minSeverity}
}

// NewSyslogCEF dials a syslog collector ("udp"/"tcp", host:port) and emits CEF
// records with a syslog priority prefix. The connection is re-dialed on failure.
func NewSyslogCEF(network, addr string, minSeverity event.Severity) (*CEFSink, error) {
	conn, err := net.DialTimeout(network, addr, 5*time.Second)
	if err != nil {
		return nil, err
	}
	s := NewCEFWriter(conn, minSeverity)
	s.network, s.addr = network, addr
	return s, nil
}

// Emit writes one CEF record.
func (s *CEFSink) Emit(e event.Event) error {
	if e.Severity < s.minSev {
		return nil
	}
	line := s.syslogPrefix(e) + FormatCEF(e) + "\n"
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := io.WriteString(s.w, line); err != nil {
		if s.redial() == nil {
			_, err2 := io.WriteString(s.w, line)
			return err2
		}
		return err
	}
	return nil
}

func (s *CEFSink) redial() error {
	if s.network == "" {
		return fmt.Errorf("cef: writer is not reconnectable")
	}
	conn, err := net.DialTimeout(s.network, s.addr, 5*time.Second)
	if err != nil {
		return err
	}
	s.w = conn
	return nil
}

// syslogPrefix is "<PRI>timestamp host " for syslog transports, empty otherwise.
func (s *CEFSink) syslogPrefix(e event.Event) string {
	if s.network == "" {
		return ""
	}
	const facilityLocal0 = 16
	pri := facilityLocal0*8 + syslogSeverity(e.Severity)
	return fmt.Sprintf("<%d>%s %s ", pri, e.Time.Format(time.Stamp), s.host)
}

// FormatCEF renders an event as a CEF:0 record (pure; unit-tested directly).
func FormatCEF(e event.Event) string {
	name := string(e.Kind)
	header := fmt.Sprintf("CEF:0|Armur|Heramba|1.0|%s|%s|%d",
		cefHeaderEscape(string(e.Kind)), cefHeaderEscape(name), cefSeverity(e.Severity))
	ext := []string{
		"src=" + cefExtEscape(e.Source),
		"request=" + cefExtEscape(e.Target),
		"cat=" + cefExtEscape(string(e.Kind)),
		fmt.Sprintf("cs1Label=Confidence cs1=%.2f", e.Confidence),
	}
	if e.Detail != "" {
		ext = append(ext, "msg="+cefExtEscape(e.Detail))
	}
	if c := e.Labels["campaign"]; c != "" {
		ext = append(ext, "cs2Label=Campaign cs2="+cefExtEscape(c))
	}
	return header + "|" + strings.Join(ext, " ")
}

// cefSeverity maps to CEF's 0..10 scale.
func cefSeverity(s event.Severity) int {
	switch s {
	case event.SeverityCritical:
		return 10
	case event.SeverityHigh:
		return 8
	case event.SeverityMedium:
		return 5
	case event.SeverityLow:
		return 3
	default:
		return 1
	}
}

// syslogSeverity maps to RFC 5424 severities (0 emerg .. 7 debug).
func syslogSeverity(s event.Severity) int {
	switch s {
	case event.SeverityCritical:
		return 2 // critical
	case event.SeverityHigh:
		return 3 // error
	case event.SeverityMedium:
		return 4 // warning
	case event.SeverityLow:
		return 5 // notice
	default:
		return 6 // informational
	}
}

func cefHeaderEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return strings.ReplaceAll(s, "|", `\|`)
}

func cefExtEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "=", `\=`)
	return strings.ReplaceAll(s, "\n", " ")
}
