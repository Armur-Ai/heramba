package classify

import (
	"testing"
	"time"
)

func browserHeaders() map[string]string {
	return map[string]string{
		"sec-fetch-mode":  "navigate",
		"accept-language": "en-US",
		"accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		"accept-encoding": "gzip, deflate, br",
	}
}

func TestNonBrowserAcceptSignal(t *testing.T) {
	c := New()
	// tool-style: Accept */*, no Accept-Encoding
	v := c.Classify(Observation{Source: "5.5.5.5", Path: "/", UserAgent: "Mozilla/5.0 custom", Headers: map[string]string{"accept": "*/*"}})
	fired := false
	for _, s := range v.Signals {
		if s.Name == "non_browser_accept" && s.Fired {
			fired = true
		}
	}
	if !fired {
		t.Fatalf("non_browser_accept should fire for */* + no encoding: %+v", v.Signals)
	}
	// browser headers: must not fire
	v2 := c.Classify(Observation{Source: "5.5.5.6", Path: "/", UserAgent: "Mozilla/5.0", Headers: browserHeaders()})
	for _, s := range v2.Signals {
		if s.Name == "non_browser_accept" && s.Fired {
			t.Fatal("non_browser_accept must not fire for a real browser")
		}
	}
}

func TestBrowserLooksHuman(t *testing.T) {
	c := New()
	v := c.Classify(Observation{
		Source: "1.1.1.1", Method: "GET", Path: "/",
		UserAgent: "Mozilla/5.0 (Macintosh) AppleWebKit/537.36",
		Headers:   browserHeaders(),
	})
	if v.Label != "human" {
		t.Fatalf("browser should be human, got %q (score %v)", v.Label, v.Score)
	}
}

func TestToolUAFlagged(t *testing.T) {
	c := New()
	v := c.Classify(Observation{Source: "2.2.2.2", Method: "GET", Path: "/", UserAgent: "curl/8.1.2"})
	if v.Score < 0.4 {
		t.Fatalf("curl should score >= 0.4, got %v", v.Score)
	}
	// empty UA + no browser signals should fire two signals.
	v2 := c.Classify(Observation{Source: "2.2.2.3", Method: "GET", Path: "/", UserAgent: ""})
	if v2.Label == "human" {
		t.Fatalf("empty UA + no browser signals should not be human: %+v", v2)
	}
}

func TestHighCadenceAndEnumerationEscalate(t *testing.T) {
	c := New()
	var last Verdict
	// 60 requests, distinct paths, no browser signals, tool UA -> agentic.
	base := time.Now()
	for i := 0; i < 60; i++ {
		last = c.Classify(Observation{
			Source: "9.9.9.9", Method: "GET",
			Path:      "/api/users/" + itoaT(i),
			UserAgent: "python-requests/2.31",
			Time:      base.Add(time.Duration(i) * 100 * time.Millisecond), // 10 rps
		})
	}
	if last.Label != "agentic-suspected" {
		t.Fatalf("fast distinct-path enumeration by a tool should be agentic-suspected, got %q (%v)", last.Label, last.Score)
	}
	// every verdict must be explainable
	if len(last.Signals) == 0 {
		t.Fatal("verdict must list signals")
	}
}

func itoaT(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
