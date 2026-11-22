package dashboard

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

// scriptSrcRe extracts the src of every <script> tag.
var scriptSrcRe = regexp.MustCompile(`(?i)<script[^>]*\bsrc=["']([^"']+)["']`)

// linkHrefRe extracts the href of every <link> tag.
var linkHrefRe = regexp.MustCompile(`(?i)<link[^>]*\bhref=["']([^"']+)["']`)

func TestSPANonEmptyAndLocalAssets(t *testing.T) {
	h := Handler()
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if len(strings.TrimSpace(body)) < 500 {
		t.Fatalf("SPA shell suspiciously short (%d bytes)", len(body))
	}
	if !strings.Contains(body, "HERAMBA") {
		t.Error("SPA does not contain the HERAMBA brand")
	}
	// Must reference the embedded vendor charting asset.
	if !strings.Contains(body, "vendor/charts.js") {
		t.Error("SPA does not reference the embedded vendor asset vendor/charts.js")
	}
	if !strings.Contains(body, "app.js") || !strings.Contains(body, "app.css") {
		t.Error("SPA does not reference local app.js / app.css")
	}
}

func isExternal(ref string) bool {
	r := strings.ToLower(strings.TrimSpace(ref))
	return strings.HasPrefix(r, "http://") ||
		strings.HasPrefix(r, "https://") ||
		strings.HasPrefix(r, "//")
}

func TestSPAHasNoExternalScriptOrStyleOrigins(t *testing.T) {
	h := Handler()
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	body := rr.Body.String()

	for _, m := range scriptSrcRe.FindAllStringSubmatch(body, -1) {
		if isExternal(m[1]) {
			t.Errorf("external script src found: %q (air-gapped build must embed everything)", m[1])
		}
	}
	for _, m := range linkHrefRe.FindAllStringSubmatch(body, -1) {
		// data: icons are fine; only reject network origins.
		if isExternal(m[1]) {
			t.Errorf("external link href found: %q", m[1])
		}
	}
}

// anchorHrefRe extracts the href of every <a> tag.
var anchorHrefRe = regexp.MustCompile(`(?i)<a[^>]*\bhref=["']([^"']+)["']`)

func TestSPAHasNoExternalAnchorOrigins(t *testing.T) {
	h := Handler()
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	body := rr.Body.String()
	for _, m := range anchorHrefRe.FindAllStringSubmatch(body, -1) {
		// in-page anchors (#...) and relative paths are fine; reject origins.
		if isExternal(m[1]) {
			t.Errorf("external anchor href found: %q (air-gapped build)", m[1])
		}
	}
}

func TestVendorExposesStackedArea(t *testing.T) {
	h := Handler()
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/vendor/charts.js", nil))
	if !strings.Contains(rr.Body.String(), "stackedArea") {
		t.Error("vendored chart library missing stackedArea (severity-over-time view)")
	}
}

func TestVendorAssetIsServedAndSubstantial(t *testing.T) {
	h := Handler()
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/vendor/charts.js", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /vendor/charts.js status = %d, want 200", rr.Code)
	}
	if rr.Body.Len() < 1000 {
		t.Errorf("vendor charts.js is only %d bytes, expected a real library", rr.Body.Len())
	}
	if !strings.Contains(rr.Body.String(), "MIT License") {
		t.Error("vendor charts.js missing its MIT license header")
	}
}
