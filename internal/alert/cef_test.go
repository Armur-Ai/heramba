package alert

import (
	"bufio"
	"bytes"
	"net"
	"strings"
	"testing"

	"github.com/Armur-Ai/heramba/internal/event"
)

func TestFormatCEFStructure(t *testing.T) {
	e := event.Deception(event.KindDecoyHit, "10.0.0.9", "/.env", "decoy hit").WithLabel("campaign", "sweep")
	got := FormatCEF(e)
	if !strings.HasPrefix(got, "CEF:0|Armur|Heramba|1.0|decoy_hit|decoy_hit|8|") {
		t.Fatalf("bad CEF header: %s", got)
	}
	for _, want := range []string{"src=10.0.0.9", "request=/.env", "cs1Label=Confidence", "cs2=sweep"} {
		if !strings.Contains(got, want) {
			t.Errorf("CEF missing %q in %s", want, got)
		}
	}
}

func TestFormatCEFEscaping(t *testing.T) {
	e := event.New(event.KindProbe, event.SeverityLow, "1.1.1.1", "/a=b", "pipe|and=eq")
	got := FormatCEF(e)
	if !strings.Contains(got, `request=/a\=b`) {
		t.Errorf("'=' in extension must be escaped: %s", got)
	}
	if !strings.Contains(got, `msg=pipe|and\=eq`) {
		t.Errorf("'=' in msg must be escaped: %s", got)
	}
}

func TestCEFWriterSeverityFloor(t *testing.T) {
	var buf bytes.Buffer
	s := NewCEFWriter(&buf, event.SeverityHigh)
	_ = s.Emit(event.New(event.KindProbe, event.SeverityLow, "x", "y", "d"))
	if buf.Len() != 0 {
		t.Fatal("below-floor event must not be written")
	}
	_ = s.Emit(event.New(event.KindEgressBlocked, event.SeverityHigh, "x", "y", "d"))
	if buf.Len() == 0 {
		t.Fatal("at-floor event must be written")
	}
}

func TestSyslogCEFOverTCP(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	got := make(chan string, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		line, _ := bufio.NewReader(c).ReadString('\n')
		got <- line
	}()

	s, err := NewSyslogCEF("tcp", ln.Addr().String(), event.SeverityInfo)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Emit(event.Deception(event.KindCanaryTriggered, "2.2.2.2", "tok", "d")); err != nil {
		t.Fatal(err)
	}
	line := <-got
	if !strings.HasPrefix(line, "<") || !strings.Contains(line, "CEF:0|Armur|Heramba") {
		t.Errorf("syslog line missing PRI/CEF: %q", line)
	}
}
