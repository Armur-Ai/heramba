package alert

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/smtp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

// httpSinkCase describes one HTTP push sink for the shared floor + payload table.
type httpSinkCase struct {
	name string
	// make builds the sink pointing at the test server base URL, floored at Medium.
	make func(base string) Sink
	// wantContains are substrings the posted (JSON) body must contain for a
	// sample event, pinning each sink's payload shape.
	wantContains []string
}

func httpSinkCases() []httpSinkCase {
	const to = time.Second
	med := event.SeverityMedium
	return []httpSinkCase{
		{"webhook", func(u string) Sink { return NewWebhook(u, to, med) }, []string{"decoy_hit"}},
		{"slack", func(u string) Sink { return NewSlack(u, to, med) }, []string{"attachments"}},
		{"teams", func(u string) Sink { return NewTeams(u, to, med) }, []string{"MessageCard"}},
		{"discord", func(u string) Sink { return NewDiscord(u, to, med) }, []string{"embeds"}},
		{"mattermost", func(u string) Sink { return NewMattermost(u, to, med) }, []string{"attachments"}},
		{"googlechat", func(u string) Sink { return NewGoogleChat(u, to, med) }, []string{"Heramba"}},
		{"sumologic", func(u string) Sink { return NewSumoLogic(u, to, med) }, []string{"decoy_hit"}},
		{"splunk", func(u string) Sink { return NewSplunk(u, "tok", to, med) }, []string{"sourcetype", "event"}},
		{"datadog", func(u string) Sink { return NewDatadog(u, "key", to, med) }, []string{"ddsource"}},
		{"loki", func(u string) Sink { return NewLoki(u, "", to, med) }, []string{"streams"}},
		{"elastic", func(u string) Sink { return NewElastic(u, "idx", "", to, med) }, []string{"@timestamp"}},
		{"gelf", func(u string) Sink { return NewGELF(u, "", to, med) }, []string{"short_message"}},
		{"axiom", func(u string) Sink { return NewAxiom(u, "ds", "tok", to, med) }, []string{"_time"}},
		{"betterstack", func(u string) Sink { return NewBetterStack(u, "tok", to, med) }, []string{`"dt"`}},
		{"honeycomb", func(u string) Sink { return NewHoneycomb(u, "ds", "key", to, med) }, []string{"severity"}},
		{"newrelic", func(u string) Sink { return NewNewRelic(u, "key", to, med) }, []string{"attributes"}},
		{"openobserve", func(u string) Sink { return NewOpenObserve(u, "org", "stream", "us", "pw", to, med) }, []string{"_timestamp"}},
		{"templatewebhook", func(u string) Sink {
			s, err := NewTemplateWebhook(u, `{"kind":"{{.Kind}}"}`, "", to, med)
			if err != nil {
				panic(err)
			}
			return s
		}, []string{"decoy_hit"}},
		{"pagerduty", func(u string) Sink {
			s := NewPagerDuty("rk", to, med)
			s.url = u
			return s
		}, []string{"routing_key"}},
		{"opsgenie", func(u string) Sink {
			s := NewOpsgenie("key", to, med)
			s.url = u
			return s
		}, []string{"priority"}},
	}
}

// captureServer records the request count and the last body posted to it.
type captureServer struct {
	mu   sync.Mutex
	hits int
	body []byte
}

func (c *captureServer) handler(w http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(r.Body)
	c.mu.Lock()
	c.hits++
	c.body = b
	c.mu.Unlock()
	w.WriteHeader(http.StatusOK)
}

func (c *captureServer) snapshot() (int, []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.hits, append([]byte(nil), c.body...)
}

func TestHTTPSinkFloorAndPayloadShape(t *testing.T) {
	below := event.New(event.KindProbe, event.SeverityLow, "1.2.3.4", "/x", "noise") // below Medium floor
	above := event.Deception(event.KindDecoyHit, "9.9.9.9", "/.env", "decoy hit")    // High, delivered

	for _, tc := range httpSinkCases() {
		t.Run(tc.name, func(t *testing.T) {
			cap := &captureServer{}
			srv := httptest.NewServer(http.HandlerFunc(cap.handler))
			defer srv.Close()

			sink := tc.make(srv.URL)

			// Below the floor: must not be delivered.
			if err := sink.Emit(below); err != nil {
				t.Fatalf("below-floor emit errored: %v", err)
			}
			if hits, _ := cap.snapshot(); hits != 0 {
				t.Fatalf("below-floor event was delivered (%d hits)", hits)
			}

			// At/above the floor: delivered exactly once with the expected shape.
			if err := sink.Emit(above); err != nil {
				t.Fatalf("above-floor emit errored: %v", err)
			}
			hits, body := cap.snapshot()
			if hits != 1 {
				t.Fatalf("expected exactly 1 delivery, got %d", hits)
			}
			if !json.Valid(body) {
				t.Fatalf("posted body is not valid JSON: %s", body)
			}
			for _, want := range tc.wantContains {
				if !strings.Contains(string(body), want) {
					t.Fatalf("payload missing %q; body=%s", want, body)
				}
			}
		})
	}
}

// recordFlusher counts Emit and Flush calls to verify Flush propagation.
type recordFlusher struct {
	emits   int
	flushes int
}

func (r *recordFlusher) Emit(event.Event) error { r.emits++; return nil }
func (r *recordFlusher) Flush() error           { r.flushes++; return nil }

func TestFlushPropagatesThroughWrappers(t *testing.T) {
	t.Run("MultiSink", func(t *testing.T) {
		rf := &recordFlusher{}
		if err := FlushAll(MultiSink{rf, Discard}); err != nil {
			t.Fatal(err)
		}
		if rf.flushes != 1 {
			t.Fatalf("MultiSink member not flushed: %d", rf.flushes)
		}
	})
	t.Run("RedactingSink", func(t *testing.T) {
		rf := &recordFlusher{}
		if err := NewRedacting(rf, 0).Flush(); err != nil {
			t.Fatal(err)
		}
		if rf.flushes != 1 {
			t.Fatalf("RedactingSink did not delegate Flush: %d", rf.flushes)
		}
	})
	t.Run("Breaker", func(t *testing.T) {
		rf := &recordFlusher{}
		if err := NewBreaker(rf, BreakerOptions{}).Flush(); err != nil {
			t.Fatal(err)
		}
		if rf.flushes != 1 {
			t.Fatalf("Breaker did not delegate Flush: %d", rf.flushes)
		}
	})
	t.Run("SwapSink", func(t *testing.T) {
		rf := &recordFlusher{}
		if err := NewSwap(rf).Flush(); err != nil {
			t.Fatal(err)
		}
		if rf.flushes != 1 {
			t.Fatalf("SwapSink did not delegate Flush: %d", rf.flushes)
		}
	})
	t.Run("non-flusher is a no-op", func(t *testing.T) {
		if err := FlushAll(Discard); err != nil {
			t.Fatalf("flushing a non-flusher sink should be a no-op, got %v", err)
		}
	})
}

func TestEmailDigestBuffersThenFlushes(t *testing.T) {
	s, err := NewEmail(EmailOptions{
		Host: "localhost", Port: 2525, From: "a@test", To: []string{"b@test"},
		MinSev: event.SeverityMedium, BatchSize: 100, // large batch so Emit buffers
	})
	if err != nil {
		t.Fatal(err)
	}
	var sends int
	var lastMsg []byte
	s.send = func(_ string, _ smtp.Auth, _ string, _ []string, msg []byte) error {
		sends++
		lastMsg = msg
		return nil
	}

	// Below the floor: ignored, nothing buffered.
	_ = s.Emit(event.New(event.KindProbe, event.SeverityLow, "x", "y", "d"))
	// Above the floor: buffered, not yet sent (batch not reached).
	_ = s.Emit(event.Deception(event.KindDecoyHit, "1.2.3.4", "/.env", "decoy"))
	if sends != 0 {
		t.Fatalf("digest should buffer until Flush, got %d sends", sends)
	}
	// Flush drains the buffered digest exactly once.
	if err := FlushAll(s); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if sends != 1 {
		t.Fatalf("expected one digest send on flush, got %d", sends)
	}
	if !strings.Contains(string(lastMsg), "decoy_hit") {
		t.Fatalf("digest missing buffered event; msg=%s", lastMsg)
	}
	// A second flush with an empty buffer is a no-op.
	if err := s.Flush(); err != nil || sends != 1 {
		t.Fatalf("empty flush should be a no-op (sends=%d err=%v)", sends, err)
	}
}
