package edge

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Armur-Ai/heramba/internal/config"
	"github.com/Armur-Ai/heramba/internal/decoy"
	"github.com/Armur-Ai/heramba/internal/event"
)

// TestEndToEndDefenseFlow drives one configured edge through a realistic mix of
// requests and asserts the full detection surface fires together: a legitimate
// browser request passes through cleanly, a tool probing a decoy is detected,
// and a planted canary used by an attacker is caught.
func TestEndToEndDefenseFlow(t *testing.T) {
	up := newUpstream(t)
	defer up.Close()
	token, _ := decoy.NewCanaryStore().Mint("prod-db")

	cfg := config.Default()
	cfg.Mode = config.ModeProtect
	cfg.Upstream = up.URL
	cfg.Decoys.Canaries = []string{token.Token}
	// Tool clients (curl, go-http-client) score ~0.65; a stricter operator lowers
	// the agentic threshold so they are flagged. (The Go HTTP client auto-adds
	// Accept-Encoding, so the non_browser_accept signal doesn't fire here.)
	cfg.Classifier.AgenticAt = 0.6
	sink := &capSink{}
	e, _ := New(cfg, sink)
	srv := httptest.NewServer(e.Handler())
	defer srv.Close()

	// 1) A real browser hitting a real path: proxied, headers normalized.
	brow, _ := http.NewRequest("GET", srv.URL+"/api/health", nil)
	brow.Header.Set("User-Agent", "Mozilla/5.0 AppleWebKit/537.36")
	brow.Header.Set("Accept", "text/html,application/xhtml+xml")
	brow.Header.Set("Accept-Encoding", "gzip, deflate, br")
	brow.Header.Set("Accept-Language", "en-US")
	brow.Header.Set("Sec-Fetch-Mode", "navigate")
	r1, err := http.DefaultClient.Do(brow)
	if err != nil {
		t.Fatal(err)
	}
	if r1.StatusCode != 200 {
		t.Fatalf("browser request should proxy 200, got %d", r1.StatusCode)
	}
	if r1.Header.Get("X-Powered-By") != "" {
		t.Fatal("protect mode should have normalized the leaky header")
	}

	// 2) A tool probing a decoy path.
	tool, _ := http.NewRequest("GET", srv.URL+"/.env", nil)
	tool.Header.Set("User-Agent", "curl/8.1.2")
	_, _ = http.DefaultClient.Do(tool)

	// 3) An attacker using the planted canary token.
	_, _ = http.Get(srv.URL + "/api/data?token=" + token.Token)

	kinds := sink.kinds()
	if kinds[event.KindDecoyHit] < 1 {
		t.Fatalf("expected a decoy_hit, got %+v", kinds)
	}
	if kinds[event.KindCanaryTriggered] < 1 {
		t.Fatalf("expected a canary_triggered, got %+v", kinds)
	}
	if kinds[event.KindAgenticTraffic] < 1 {
		t.Fatalf("expected an agentic_traffic detection (curl probe), got %+v", kinds)
	}
}
