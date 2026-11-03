package deception

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// client is a tiny synthetic HTTP client that carries the deception session id
// across requests, so it behaves like an attacker navigating a real app.
type client struct {
	t      *testing.T
	srv    *httptest.Server
	sessID string
}

func (c *client) do(method, path string, body any) (int, map[string]any) {
	c.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req, err := http.NewRequest(method, c.srv.URL+path, &buf)
	if err != nil {
		c.t.Fatalf("new request: %v", err)
	}
	if c.sessID != "" {
		req.Header.Set("X-Deception-Session", c.sessID)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if s := resp.Header.Get("X-Deception-Session"); s != "" {
		c.sessID = s
	}
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

// TestSyntheticClientNavigatesCoherentApp is the CP3 acceptance test: a
// synthetic client logs in and navigates a coherent fake application across
// multiple requests (list → detail → create → read-back) with consistent state
// and without touching any production data.
func TestSyntheticClientNavigatesCoherentApp(t *testing.T) {
	w := GenerateWorld(42, 6, 4, 10)
	app := NewApp(w)
	var interactions []Interaction
	app.OnInteraction = func(in Interaction) { interactions = append(interactions, in) }
	srv := httptest.NewServer(app)
	defer srv.Close()
	c := &client{t: t, srv: srv}

	// unauthenticated access is refused
	if st, _ := c.do(http.MethodGet, "/api/projects", nil); st != http.StatusUnauthorized {
		t.Fatalf("expected 401 before login, got %d", st)
	}

	// log in as an admin so we can see projects (admin can access all)
	var adminUser string
	for _, id := range w.sortedUserIDs() {
		if w.Users[id].Role == RoleAdmin {
			adminUser = w.Users[id].Username
			break
		}
	}
	st, body := c.do(http.MethodPost, "/api/login", map[string]string{"username": adminUser})
	if st != http.StatusOK {
		t.Fatalf("login failed: %d %v", st, body)
	}

	// list projects — coherent, non-empty for an admin
	st, body = c.do(http.MethodGet, "/api/projects", nil)
	if st != http.StatusOK {
		t.Fatalf("list projects: %d", st)
	}
	projects, _ := body["projects"].([]any)
	if len(projects) == 0 {
		t.Fatal("admin should see projects")
	}
	first := projects[0].(map[string]any)
	pid := first["id"].(string)

	// get that project — consistent id echoes back
	st, body = c.do(http.MethodGet, "/api/projects/"+pid, nil)
	if st != http.StatusOK || body["id"] != pid {
		t.Fatalf("project detail inconsistent: %d %v", st, body)
	}

	// create a resource, then read it back in the SAME session (coherent state)
	st, body = c.do(http.MethodPost, "/api/projects/"+pid+"/resources",
		map[string]string{"kind": "document", "name": "loot"})
	if st != http.StatusCreated {
		t.Fatalf("create resource: %d %v", st, body)
	}
	rid := body["id"].(string)
	st, body = c.do(http.MethodGet, "/api/resources/"+rid, nil)
	if st != http.StatusOK || body["name"] != "loot" {
		t.Fatalf("read-after-write failed: %d %v", st, body)
	}

	// telemetry recorded the login + the navigation, all attributed to one session
	if len(interactions) < 5 {
		t.Fatalf("expected telemetry for each request, got %d", len(interactions))
	}
	sawLogin := false
	for _, in := range interactions {
		if in.SessionID != c.sessID {
			t.Fatalf("interaction not attributed to the session: %q vs %q", in.SessionID, c.sessID)
		}
		if in.Note == "login" {
			sawLogin = true
		}
	}
	if !sawLogin {
		t.Fatal("login interaction not recorded")
	}
}

// TestAuthorizationViolationDetected shows a synthetic BOLA/IDOR: a low-priv
// user reaching a project they don't belong to yields a recorded authz
// violation, while an authorized access does not.
func TestAuthorizationViolationDetected(t *testing.T) {
	w := GenerateWorld(7, 8, 6, 12)
	app := NewApp(w)
	var violations int
	app.OnInteraction = func(in Interaction) {
		if in.Note == "authz_violation" {
			violations++
		}
	}
	srv := httptest.NewServer(app)
	defer srv.Close()

	// find a viewer and a project they are NOT a member of
	var viewer *User
	for _, id := range w.sortedUserIDs() {
		if w.Users[id].Role == RoleViewer {
			viewer = w.Users[id]
			break
		}
	}
	if viewer == nil {
		t.Skip("no viewer in generated world")
	}
	var forbidden, allowed string
	for _, id := range w.sortedProjectIDs() {
		if w.CanAccessProject(viewer.ID, id) {
			allowed = id
		} else {
			forbidden = id
		}
	}

	c := &client{t: t, srv: srv}
	c.do(http.MethodPost, "/api/login", map[string]string{"username": viewer.Username})

	if forbidden != "" {
		if st, _ := c.do(http.MethodGet, "/api/projects/"+forbidden, nil); st != http.StatusForbidden {
			t.Fatalf("expected 403 on forbidden project, got %d", st)
		}
		if violations == 0 {
			t.Fatal("authz violation not recorded")
		}
	}
	if allowed != "" {
		before := violations
		if st, _ := c.do(http.MethodGet, "/api/projects/"+allowed, nil); st != http.StatusOK {
			t.Fatalf("expected 200 on allowed project, got %d", st)
		}
		if violations != before {
			t.Fatal("authorized access wrongly flagged as a violation")
		}
	}
}

// TestWorldIsDeterministicAndSynthetic verifies reproducible seeds and the
// isolation invariant (everything tagged synthetic, no real domains).
func TestWorldIsDeterministicAndSynthetic(t *testing.T) {
	a := GenerateWorld(99, 5, 3, 8)
	b := GenerateWorld(99, 5, 3, 8)
	if len(a.Users) != len(b.Users) || len(a.Projects) != len(b.Projects) {
		t.Fatal("same seed produced different sizes")
	}
	for id, ua := range a.Users {
		ub, ok := b.Users[id]
		if !ok || ua.Username != ub.Username || ua.APIToken != ub.APIToken {
			t.Fatalf("seed not deterministic for %s", id)
		}
		if !ua.Synthetic {
			t.Fatalf("user %s not tagged synthetic", id)
		}
		if !strings.HasSuffix(ua.Email, ".test") {
			t.Fatalf("user email %q is not a reserved test domain", ua.Email)
		}
	}
	// every token is discoverable for canary registration
	if len(a.AllTokens()) == 0 {
		t.Fatal("no canary tokens exposed for registration")
	}
}

// TestSessionsAreIndependent confirms per-session overlays don't leak between
// attackers and the base world stays immutable.
func TestSessionsAreIndependent(t *testing.T) {
	w := GenerateWorld(3, 4, 2, 4)
	app := NewApp(w)
	srv := httptest.NewServer(app)
	defer srv.Close()

	admin := ""
	pid := w.sortedProjectIDs()[0]
	for _, id := range w.sortedUserIDs() {
		if w.Users[id].Role == RoleAdmin {
			admin = w.Users[id].Username
		}
	}
	c1 := &client{t: t, srv: srv}
	c1.do(http.MethodPost, "/api/login", map[string]string{"username": admin})
	st, body := c1.do(http.MethodPost, "/api/projects/"+pid+"/resources", map[string]string{"name": "c1-only"})
	if st != http.StatusCreated {
		t.Fatalf("c1 create: %d", st)
	}
	rid := body["id"].(string)

	// a second session must NOT see c1's session-local resource
	c2 := &client{t: t, srv: srv}
	c2.do(http.MethodPost, "/api/login", map[string]string{"username": admin})
	if st, _ := c2.do(http.MethodGet, "/api/resources/"+rid, nil); st != http.StatusNotFound {
		t.Fatalf("session overlay leaked across sessions: got %d, want 404", st)
	}
	// base world untouched
	if _, ok := w.Resources[rid]; ok {
		t.Fatal("session-local resource leaked into the base world")
	}
}
