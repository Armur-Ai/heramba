// Package classify is Heramba's behavioral agentic-traffic classifier. It scores
// how automated/agentic a request source looks using DETERMINISTIC, explainable
// signals only — request cadence, client fingerprint, missing browser signals,
// and enumeration shape. It never calls an LLM: the critical detection path must
// be fast, offline, reproducible, and un-foolable by prompt tricks. (An AI-
// augmented classifier is a separate commercial plugin that layers on top.)
//
// Every verdict lists the signals that fired, so an operator can see exactly why
// a source was flagged.
package classify

import (
	"strings"
	"sync"
	"time"
)

// Observation is the minimal view of a request the classifier needs.
type Observation struct {
	Source    string // client IP / identity
	Method    string
	Path      string
	UserAgent string
	Headers   map[string]string // lower-cased header name -> value (subset)
	Time      time.Time
}

// Signal is one deterministic indicator and whether it fired.
type Signal struct {
	Name   string  `json:"name"`
	Weight float64 `json:"weight"`
	Fired  bool    `json:"fired"`
	Detail string  `json:"detail,omitempty"`
}

// Verdict is the classifier output: a 0..1 score, a label, and the signals.
type Verdict struct {
	Score   float64  `json:"score"`
	Label   string   `json:"label"` // human | tool | agentic-suspected
	Signals []Signal `json:"signals"`
}

// cadenceWindow is the sliding window over which request rate is measured.
const cadenceWindow = 10 * time.Second

// Options are the deterministic thresholds. Zero fields fall back to defaults.
type Options struct {
	CadenceRPS   float64 // sustained rate above this (req/s) fires high_cadence
	EnumDistinct int     // distinct paths in-window at/above this fires enumeration
	ToolAt       float64 // score >= this => "tool"
	AgenticAt    float64 // score >= this => "agentic-suspected"
}

// DefaultOptions are the conservative built-in thresholds.
func DefaultOptions() Options {
	return Options{CadenceRPS: 5.0, EnumDistinct: 12, ToolAt: 0.4, AgenticAt: 0.7}
}

// withDefaults fills any zero field from DefaultOptions.
func (o Options) withDefaults() Options {
	d := DefaultOptions()
	if o.CadenceRPS == 0 {
		o.CadenceRPS = d.CadenceRPS
	}
	if o.EnumDistinct == 0 {
		o.EnumDistinct = d.EnumDistinct
	}
	if o.ToolAt == 0 {
		o.ToolAt = d.ToolAt
	}
	if o.AgenticAt == 0 {
		o.AgenticAt = d.AgenticAt
	}
	return o
}

// knownToolAgents are substrings of non-browser client UAs.
var knownToolAgents = []string{
	"curl/", "wget/", "python-requests", "python-urllib", "go-http-client",
	"httpx", "nuclei", "sqlmap", "nikto", "zgrab", "masscan", "nmap",
	"java/", "okhttp", "libwww-perl", "scrapy", "httpclient", "aiohttp",
}

// Classifier is stateful per source (for cadence/enumeration). Safe for
// concurrent use.
type Classifier struct {
	mu        sync.Mutex
	perSource map[string]*sourceState
	opts      Options
}

type sourceState struct {
	times []time.Time
	paths map[string]struct{}
}

// New returns a ready classifier with default thresholds.
func New() *Classifier { return NewWithOptions(DefaultOptions()) }

// NewWithOptions returns a classifier with custom thresholds (zero fields use
// defaults).
func NewWithOptions(opts Options) *Classifier {
	return &Classifier{perSource: map[string]*sourceState{}, opts: opts.withDefaults()}
}

// Classify scores an observation and updates per-source state.
func (c *Classifier) Classify(o Observation) Verdict {
	if o.Time.IsZero() {
		o.Time = time.Now().UTC()
	}
	cadence, distinct := c.track(o)

	ua := strings.ToLower(o.UserAgent)
	signals := []Signal{
		toolUASignal(ua),
		missingBrowserSignal(ua, o.Headers),
		acceptSignal(o.Headers),
		uaHeaderMismatchSignal(ua, o.Headers),
		cadenceSignal(cadence, c.opts.CadenceRPS),
		enumerationSignal(distinct, c.opts.EnumDistinct),
	}

	var score float64
	for _, s := range signals {
		if s.Fired {
			score += s.Weight
		}
	}
	if score > 1 {
		score = 1
	}
	return Verdict{Score: round2(score), Label: c.labelFor(score), Signals: signals}
}

func (c *Classifier) track(o Observation) (cadenceRPS float64, distinctPaths int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	st := c.perSource[o.Source]
	if st == nil {
		st = &sourceState{paths: map[string]struct{}{}}
		c.perSource[o.Source] = st
	}
	cutoff := o.Time.Add(-cadenceWindow)
	kept := st.times[:0]
	for _, t := range st.times {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	st.times = append(kept, o.Time)
	st.paths[o.Path] = struct{}{}
	rps := float64(len(st.times)) / cadenceWindow.Seconds()
	return rps, len(st.paths)
}

func toolUASignal(ua string) Signal {
	s := Signal{Name: "tool_user_agent", Weight: 0.4}
	if ua == "" {
		s.Fired, s.Detail = true, "empty User-Agent"
		return s
	}
	for _, t := range knownToolAgents {
		if strings.Contains(ua, t) {
			s.Fired, s.Detail = true, "non-browser client: "+t
			return s
		}
	}
	return s
}

func missingBrowserSignal(ua string, h map[string]string) Signal {
	s := Signal{Name: "missing_browser_signals", Weight: 0.25}
	looksBrowser := strings.Contains(ua, "mozilla") || strings.Contains(ua, "applewebkit")
	_, hasSecFetch := h["sec-fetch-mode"]
	_, hasAcceptLang := h["accept-language"]
	if !looksBrowser && !hasSecFetch && !hasAcceptLang {
		s.Fired, s.Detail = true, "no Sec-Fetch-*, no Accept-Language, UA not a browser"
	}
	return s
}

// acceptSignal fires when the request's Accept headers look non-browser: a
// browser sends a rich Accept (containing text/html) and an Accept-Encoding;
// most tools send "*/*" or nothing and omit Accept-Encoding.
func acceptSignal(h map[string]string) Signal {
	s := Signal{Name: "non_browser_accept", Weight: 0.15}
	accept := h["accept"]
	_, hasEnc := h["accept-encoding"]
	browserishAccept := strings.Contains(accept, "text/html")
	if !browserishAccept && !hasEnc {
		s.Fired, s.Detail = true, "no text/html Accept and no Accept-Encoding"
	}
	return s
}

// uaHeaderMismatchSignal fires when a request's User-Agent claims a modern
// Chromium browser but the request is missing the header fingerprint that real
// Chromium always sends — the client-hint (`Sec-CH-UA`) and `Sec-Fetch-*`
// headers. Spoofing a browser UA string is trivial; replicating the full,
// consistent header fingerprint is not, so a UA/header mismatch is a strong,
// hard-to-forge agentic tell. This complements missing_browser_signals (which
// fires when the UA is *not* a browser); this one catches a UA that *pretends*
// to be one.
//
// True wire-level fingerprinting (JA3/JA4 TLS, raw header ordering) needs
// capture below net/http, which discards header order — tracked separately.
func uaHeaderMismatchSignal(ua string, h map[string]string) Signal {
	s := Signal{Name: "ua_header_mismatch", Weight: 0.3}
	claimsChromium := strings.Contains(ua, "chrome/") || strings.Contains(ua, "chromium") || strings.Contains(ua, "edg/")
	if !claimsChromium {
		return s // only meaningful when the UA claims a client whose fingerprint we know
	}
	_, hasCHUA := h["sec-ch-ua"]
	_, hasSecFetch := h["sec-fetch-mode"]
	if !hasCHUA && !hasSecFetch {
		s.Fired, s.Detail = true, "UA claims Chromium but sends no Sec-CH-UA / Sec-Fetch headers (spoofed UA fingerprint)"
	}
	return s
}

func cadenceSignal(rps, threshold float64) Signal {
	s := Signal{Name: "high_cadence", Weight: 0.25}
	if rps > threshold {
		s.Fired, s.Detail = true, "sustained request rate above human threshold"
	}
	return s
}

func enumerationSignal(distinct, threshold int) Signal {
	s := Signal{Name: "enumeration", Weight: 0.2}
	if distinct >= threshold {
		s.Fired, s.Detail = true, "many distinct paths in a short window"
	}
	return s
}

func (c *Classifier) labelFor(score float64) string {
	switch {
	case score >= c.opts.AgenticAt:
		return "agentic-suspected"
	case score >= c.opts.ToolAt:
		return "tool"
	default:
		return "human"
	}
}

func round2(f float64) float64 { return float64(int(f*100+0.5)) / 100 }
