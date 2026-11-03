package edge

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Armur-Ai/heramba/internal/config"
	"github.com/Armur-Ai/heramba/internal/event"
)

func htmlUpstream(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><body><h1>app</h1></body></html>"))
	}))
}

func TestCounterInjectsForAgenticSource(t *testing.T) {
	up := htmlUpstream(t)
	defer up.Close()
	cfg := config.Default()
	cfg.Mode = config.ModeProtect
	cfg.Upstream = up.URL
	cfg.Counter.Enabled = true
	cfg.Classifier.AgenticAt = 0.6 // ensure a tool client is flagged
	sink := &capSink{}
	e, _ := New(cfg, sink)
	srv := httptest.NewServer(e.Handler())
	defer srv.Close()

	// tool client -> flagged agentic -> lure injected
	req, _ := http.NewRequest("GET", srv.URL+"/", nil)
	req.Header.Set("User-Agent", "curl/8.1.2")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "assessment-note[") {
		t.Fatalf("expected a counter-agent lure in the HTML, got: %s", body)
	}
	if !strings.Contains(string(body), "<h1>app</h1>") {
		t.Fatal("original content must be preserved")
	}
	if sink.kinds()[event.KindCounterDeployed] < 1 {
		t.Fatalf("expected a counter_deployed event, got %+v", sink.kinds())
	}
}

func TestCounterSilentForHumanAndObserve(t *testing.T) {
	up := htmlUpstream(t)
	defer up.Close()

	// human source in protect: no injection
	cfg := config.Default()
	cfg.Mode = config.ModeProtect
	cfg.Upstream = up.URL
	cfg.Counter.Enabled = true
	e, _ := New(cfg, &capSink{})
	srv := httptest.NewServer(e.Handler())
	defer srv.Close()
	req, _ := http.NewRequest("GET", srv.URL+"/", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 AppleWebKit/537.36")
	req.Header.Set("Accept", "text/html")
	req.Header.Set("Accept-Language", "en-US")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	resp, _ := http.DefaultClient.Do(req)
	body, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(body), "assessment-note[") {
		t.Fatal("human source must NOT get a lure")
	}

	// observe mode: never injects even for a tool
	ocfg := config.Default()
	ocfg.Upstream = up.URL
	ocfg.Counter.Enabled = true // ignored in observe
	oe, _ := New(ocfg, &capSink{})
	osrv := httptest.NewServer(oe.Handler())
	defer osrv.Close()
	oreq, _ := http.NewRequest("GET", osrv.URL+"/", nil)
	oreq.Header.Set("User-Agent", "curl/8.1.2")
	oresp, _ := http.DefaultClient.Do(oreq)
	obody, _ := io.ReadAll(oresp.Body)
	if strings.Contains(string(obody), "assessment-note[") {
		t.Fatal("observe mode must never inject")
	}
}
