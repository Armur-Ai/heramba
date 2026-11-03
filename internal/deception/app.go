package deception

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// Interaction is one recorded touch of the deception app. It is the telemetry
// the intelligence plane consumes (convertible to evidence.SecurityEvent): a
// synthetic-app interaction is, by construction, an attacker interaction.
type Interaction struct {
	Time      time.Time `json:"time"`
	SessionID string    `json:"session_id"`
	SourceKey string    `json:"source_key"`
	Method    string    `json:"method"`
	Path      string    `json:"path"`
	Status    int       `json:"status"`
	UserID    string    `json:"user_id,omitempty"`
	Note      string    `json:"note,omitempty"` // e.g. "login", "authz_violation"
	CanaryHit string    `json:"canary_hit,omitempty"`
}

// App is the stateful deception application. It answers only from its synthetic
// World and per-session overlays — it shares no storage with any real upstream.
// It implements http.Handler so it can be mounted directly or behind the edge.
type App struct {
	World    *World
	Sessions *SessionStore
	// OnInteraction, if set, is called for every request with the recorded
	// interaction. Heramba wires this to the evidence pipeline.
	OnInteraction func(Interaction)
	// SessionHeader/SessionCookie name where the opaque session id is carried.
	SessionHeader string
}

// NewApp builds a deception app over a world.
func NewApp(w *World) *App {
	return &App{
		World:         w,
		Sessions:      NewSessionStore(time.Hour),
		SessionHeader: "X-Deception-Session",
	}
}

func (a *App) emit(in Interaction) {
	if a.OnInteraction != nil {
		a.OnInteraction(in)
	}
}

func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	sessID := r.Header.Get(a.SessionHeader)
	sess := a.Sessions.Get(sessID, clientKey(r))
	w.Header().Set(a.SessionHeader, sess.ID)
	w.Header().Set("Content-Type", "application/json")

	in := Interaction{
		Time:      time.Now().UTC(),
		SessionID: sess.ID,
		SourceKey: sess.SourceKey,
		Method:    r.Method,
		Path:      r.URL.Path,
	}

	status, note, canary := a.route(w, r, sess)
	in.Status = status
	in.Note = note
	in.CanaryHit = canary
	in.UserID = sess.UserID
	a.emit(in)
}

// route dispatches a request and returns (status, note, canaryHit). It writes
// the response body. Keeping routing explicit (no external router) keeps the
// dependency surface minimal per the engineering mandate.
func (a *App) route(w http.ResponseWriter, r *http.Request, sess *Session) (int, string, string) {
	p := strings.TrimRight(r.URL.Path, "/")
	switch {
	case r.Method == http.MethodPost && p == "/api/login":
		return a.handleLogin(w, r, sess)
	case r.Method == http.MethodGet && p == "/api/me":
		return a.handleMe(w, sess)
	case r.Method == http.MethodGet && p == "/api/projects":
		return a.handleListProjects(w, sess)
	case r.Method == http.MethodGet && strings.HasPrefix(p, "/api/projects/") && strings.HasSuffix(p, "/resources"):
		return a.handleListResources(w, sess, segBetween(p, "/api/projects/", "/resources"))
	case r.Method == http.MethodPost && strings.HasPrefix(p, "/api/projects/") && strings.HasSuffix(p, "/resources"):
		return a.handleCreateResource(w, r, sess, segBetween(p, "/api/projects/", "/resources"))
	case r.Method == http.MethodGet && strings.HasPrefix(p, "/api/projects/"):
		return a.handleGetProject(w, sess, strings.TrimPrefix(p, "/api/projects/"))
	case r.Method == http.MethodGet && strings.HasPrefix(p, "/api/resources/"):
		return a.handleGetResource(w, sess, strings.TrimPrefix(p, "/api/resources/"))
	default:
		return a.writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"}), "", ""
	}
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request, sess *Session) (int, string, string) {
	var body struct {
		Username string `json:"username"`
		Token    string `json:"token"`
	}
	_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&body)

	var u *User
	var ok bool
	canary := ""
	if body.Token != "" {
		u, ok = a.World.UserByToken(body.Token)
		if ok {
			canary = body.Token // a synthetic token was used — canary signal
		}
	}
	if !ok && body.Username != "" {
		u, ok = a.World.UserByUsername(body.Username)
	}
	if !ok {
		return a.writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"}), "login_fail", canary
	}
	sess.UserID = u.ID
	return a.writeJSON(w, http.StatusOK, map[string]any{
		"token":   u.APIToken,
		"user":    publicUser(u),
		"message": "authenticated",
	}), "login", canary
}

func (a *App) handleMe(w http.ResponseWriter, sess *Session) (int, string, string) {
	if !sess.authed() {
		return a.unauth(w), "", ""
	}
	u := a.World.Users[sess.UserID]
	return a.writeJSON(w, http.StatusOK, publicUser(u)), "", ""
}

func (a *App) handleListProjects(w http.ResponseWriter, sess *Session) (int, string, string) {
	if !sess.authed() {
		return a.unauth(w), "", ""
	}
	var out []map[string]any
	for _, id := range a.World.sortedProjectIDs() {
		if a.World.CanAccessProject(sess.UserID, id) {
			p := a.World.Projects[id]
			out = append(out, map[string]any{"id": p.ID, "name": p.Name, "owner_id": p.OwnerID})
		}
	}
	return a.writeJSON(w, http.StatusOK, map[string]any{"projects": out}), "", ""
}

func (a *App) handleGetProject(w http.ResponseWriter, sess *Session, id string) (int, string, string) {
	if !sess.authed() {
		return a.unauth(w), "", ""
	}
	p, ok := a.World.Projects[id]
	if !ok {
		return a.writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such project"}), "", ""
	}
	if !a.World.CanAccessProject(sess.UserID, id) {
		// A synthetic authorization violation: authed user reaching a project
		// they have no grant to. This is the multi-request IDOR/BOLA signal.
		return a.writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"}), "authz_violation", ""
	}
	return a.writeJSON(w, http.StatusOK, p), "", ""
}

func (a *App) handleListResources(w http.ResponseWriter, sess *Session, projectID string) (int, string, string) {
	if !sess.authed() {
		return a.unauth(w), "", ""
	}
	if !a.World.CanAccessProject(sess.UserID, projectID) {
		return a.writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"}), "authz_violation", ""
	}
	var out []*Resource
	for _, r := range a.World.Resources {
		if r.ProjectID == projectID {
			out = append(out, r)
		}
	}
	// include session-created resources for this project
	sess.mu.Lock()
	for _, r := range sess.created {
		if r.ProjectID == projectID {
			out = append(out, r)
		}
	}
	sess.mu.Unlock()
	return a.writeJSON(w, http.StatusOK, map[string]any{"resources": out}), "", ""
}

func (a *App) handleCreateResource(w http.ResponseWriter, r *http.Request, sess *Session, projectID string) (int, string, string) {
	if !sess.authed() {
		return a.unauth(w), "", ""
	}
	if !a.World.CanAccessProject(sess.UserID, projectID) {
		return a.writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"}), "authz_violation", ""
	}
	var body struct {
		Kind string `json:"kind"`
		Name string `json:"name"`
	}
	_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&body)
	res := &Resource{
		ID:        "res_sess_" + sess.ID[4:12],
		ProjectID: projectID,
		Kind:      firstNonEmpty(body.Kind, "document"),
		Name:      firstNonEmpty(body.Name, "untitled"),
		Data:      map[string]string{"note": "synthetic placeholder"},
		Synthetic: true,
	}
	sess.addResource(res)
	return a.writeJSON(w, http.StatusCreated, res), "create_resource", ""
}

func (a *App) handleGetResource(w http.ResponseWriter, sess *Session, id string) (int, string, string) {
	if !sess.authed() {
		return a.unauth(w), "", ""
	}
	// session-local first (coherent read-after-write), then base world
	if r, ok := sess.resource(id); ok {
		return a.writeJSON(w, http.StatusOK, r), "", ""
	}
	r, ok := a.World.Resources[id]
	if !ok {
		return a.writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such resource"}), "", ""
	}
	if !a.World.CanAccessProject(sess.UserID, r.ProjectID) {
		return a.writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"}), "authz_violation", ""
	}
	canary := ""
	if v := r.Data["value"]; v != "" {
		canary = v // serving a synthetic secret — if it later egresses, it's a canary
	}
	return a.writeJSON(w, http.StatusOK, r), "", canary
}

func (a *App) unauth(w http.ResponseWriter) int {
	return a.writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
}

func (a *App) writeJSON(w http.ResponseWriter, status int, v any) int {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
	return status
}

func publicUser(u *User) map[string]any {
	return map[string]any{"id": u.ID, "username": u.Username, "email": u.Email, "role": u.Role}
}

func clientKey(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i > 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	host := r.RemoteAddr
	if i := strings.LastIndexByte(host, ':'); i > 0 {
		host = host[:i]
	}
	return host
}

func segBetween(p, prefix, suffix string) string {
	p = strings.TrimPrefix(p, prefix)
	return strings.TrimSuffix(p, suffix)
}

func firstNonEmpty(xs ...string) string {
	for _, x := range xs {
		if strings.TrimSpace(x) != "" {
			return x
		}
	}
	return ""
}
