package edge

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Armur-Ai/heramba/internal/config"
	"github.com/Armur-Ai/heramba/internal/decoy"
	"github.com/Armur-Ai/heramba/internal/event"
)

// echoUpstream reads and echoes the request body, so a test can prove the body
// survived canary scanning intact.
func echoUpstream(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_, _ = w.Write(b)
	}))
}

func TestCanaryInBodyDetectedAndBodyPreserved(t *testing.T) {
	up := echoUpstream(t)
	defer up.Close()
	token, _ := decoy.NewCanaryStore().Mint("db")
	cfg := config.Default()
	cfg.Upstream = up.URL
	cfg.Decoys.Canaries = []string{token.Token}
	sink := &capSink{}
	e, _ := New(cfg, sink)
	srv := httptest.NewServer(e.Handler())
	defer srv.Close()

	body := `{"note":"stolen key","key":"` + token.Token + `","tail":"xyz"}`
	resp, err := http.Post(srv.URL+"/api/import", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(resp.Body)

	if sink.kinds()[event.KindCanaryTriggered] != 1 {
		t.Fatalf("canary in body should be detected, got %+v", sink.kinds())
	}
	if string(got) != body {
		t.Fatalf("body must survive scanning intact:\n want %q\n got  %q", body, string(got))
	}
}
