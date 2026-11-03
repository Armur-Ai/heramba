package deception

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// Session is per-client state in the deception app. It tracks who the client
// authenticated as (within the synthetic world) and any resources they created
// during the session, as an overlay on the shared World — so each attacker sees
// a coherent, independent view and the base world stays immutable and
// reproducible.
type Session struct {
	ID        string
	SourceKey string
	UserID    string // "" until authenticated
	CreatedAt time.Time
	LastSeen  time.Time

	mu      sync.Mutex
	created map[string]*Resource // session-local resources, id → resource
}

func (s *Session) authed() bool { return s.UserID != "" }

func (s *Session) addResource(r *Resource) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.created == nil {
		s.created = map[string]*Resource{}
	}
	s.created[r.ID] = r
}

func (s *Session) resource(id string) (*Resource, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.created[id]
	return r, ok
}

// SessionStore holds live deception sessions, keyed by an opaque session id.
type SessionStore struct {
	mu       sync.Mutex
	sessions map[string]*Session
	ttl      time.Duration
}

// NewSessionStore returns a store expiring idle sessions after ttl.
func NewSessionStore(ttl time.Duration) *SessionStore {
	if ttl <= 0 {
		ttl = time.Hour
	}
	return &SessionStore{sessions: map[string]*Session{}, ttl: ttl}
}

// Get returns an existing session or creates a new one for the id. An empty id
// mints a fresh session with a new id.
func (st *SessionStore) Get(id, sourceKey string) *Session {
	st.mu.Lock()
	defer st.mu.Unlock()
	now := time.Now().UTC()
	if id != "" {
		if s, ok := st.sessions[id]; ok && now.Sub(s.LastSeen) < st.ttl {
			s.LastSeen = now
			return s
		}
	}
	s := &Session{
		ID:        newSessionID(),
		SourceKey: sourceKey,
		CreatedAt: now,
		LastSeen:  now,
		created:   map[string]*Resource{},
	}
	st.sessions[s.ID] = s
	return s
}

// Len reports the number of live sessions (for telemetry/tests).
func (st *SessionStore) Len() int {
	st.mu.Lock()
	defer st.mu.Unlock()
	return len(st.sessions)
}

func newSessionID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return "ses_" + hex.EncodeToString(b[:])
}
