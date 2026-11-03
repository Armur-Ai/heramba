// Package deception implements Heramba's stateful deception plane: a coherent,
// seeded, entirely synthetic application that an attacker can navigate across
// many requests. Unlike a static decoy path (internal/decoy), this is a small
// fake API with consistent identifiers, permissions, and session-scoped state —
// so an autonomous agent that probes it gets a believable, internally
// consistent world to waste its effort on while every interaction is recorded
// as high-confidence evidence.
//
// Two hard isolation rules, enforced structurally:
//
//   - The world is *generated* from a seed. No production data is ever copied
//     in. Every record is tagged Synthetic and carries no real PII.
//   - The deception app shares no storage with any real upstream. It answers
//     from its own in-memory World only.
package deception

import (
	"fmt"
	"math/rand"
	"sort"
	"time"
)

// Role is a synthetic authorization role.
type Role string

const (
	RoleViewer Role = "viewer"
	RoleMember Role = "member"
	RoleAdmin  Role = "admin"
)

// User is a synthetic user. Synthetic is always true — it is a structural tag
// so no code path can ever confuse a decoy user with a real one.
type User struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	Role      Role   `json:"role"`
	APIToken  string `json:"api_token"` // a plantable canary when leaked
	Synthetic bool   `json:"synthetic"`
}

// Project is a synthetic project owned by a user, with member access.
type Project struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	OwnerID   string   `json:"owner_id"`
	MemberIDs []string `json:"member_ids"`
	Synthetic bool     `json:"synthetic"`
}

// Resource is a synthetic resource within a project (a document, secret, etc.).
type Resource struct {
	ID        string            `json:"id"`
	ProjectID string            `json:"project_id"`
	Kind      string            `json:"kind"`
	Name      string            `json:"name"`
	Data      map[string]string `json:"data"`
	Synthetic bool              `json:"synthetic"`
}

// World is a self-consistent synthetic application state, generated from a seed
// so it is fully reproducible. It is the single source of truth the deception
// app answers from.
type World struct {
	Seed      int64
	CreatedAt time.Time
	Users     map[string]*User
	Projects  map[string]*Project
	Resources map[string]*Resource

	byUsername map[string]*User
	byToken    map[string]*User
}

// GenerateWorld builds a deterministic synthetic world from a seed. The same
// seed always yields byte-identical structure, which makes deception labs and
// replay reproducible.
func GenerateWorld(seed int64, nUsers, nProjects, nResources int) *World {
	rng := rand.New(rand.NewSource(seed))
	w := &World{
		Seed:       seed,
		CreatedAt:  time.Unix(1_700_000_000+seed, 0).UTC(), // deterministic, not time.Now
		Users:      map[string]*User{},
		Projects:   map[string]*Project{},
		Resources:  map[string]*Resource{},
		byUsername: map[string]*User{},
		byToken:    map[string]*User{},
	}
	if nUsers < 2 {
		nUsers = 2
	}
	roles := []Role{RoleViewer, RoleMember, RoleAdmin}
	for i := 0; i < nUsers; i++ {
		u := &User{
			ID:        fmt.Sprintf("usr_%04d", i),
			Username:  synthName(rng, "user"),
			Role:      roles[i%len(roles)],
			APIToken:  synthToken(rng),
			Synthetic: true,
		}
		u.Email = u.Username + "@example-corp.test" // RFC 2606 reserved TLD; never real
		w.Users[u.ID] = u
		w.byUsername[u.Username] = u
		w.byToken[u.APIToken] = u
	}
	userIDs := w.sortedUserIDs()
	for i := 0; i < nProjects; i++ {
		owner := userIDs[rng.Intn(len(userIDs))]
		p := &Project{
			ID:        fmt.Sprintf("prj_%04d", i),
			Name:      synthName(rng, "project"),
			OwnerID:   owner,
			MemberIDs: []string{owner},
			Synthetic: true,
		}
		// add 0..2 extra members
		for e := 0; e < rng.Intn(3); e++ {
			m := userIDs[rng.Intn(len(userIDs))]
			if m != owner {
				p.MemberIDs = append(p.MemberIDs, m)
			}
		}
		w.Projects[p.ID] = p
	}
	projIDs := w.sortedProjectIDs()
	kinds := []string{"document", "secret", "dataset", "config"}
	for i := 0; i < nResources && len(projIDs) > 0; i++ {
		pid := projIDs[rng.Intn(len(projIDs))]
		r := &Resource{
			ID:        fmt.Sprintf("res_%04d", i),
			ProjectID: pid,
			Kind:      kinds[rng.Intn(len(kinds))],
			Name:      synthName(rng, "resource"),
			Data:      map[string]string{"note": "synthetic placeholder"},
			Synthetic: true,
		}
		if r.Kind == "secret" {
			// A fake secret value doubles as a canary when exfiltrated.
			r.Data["value"] = synthToken(rng)
		}
		w.Resources[r.ID] = r
	}
	return w
}

// UserByUsername looks up a synthetic user for login.
func (w *World) UserByUsername(name string) (*User, bool) { u, ok := w.byUsername[name]; return u, ok }

// UserByToken looks up a synthetic user by API token (canary detection).
func (w *World) UserByToken(tok string) (*User, bool) { u, ok := w.byToken[tok]; return u, ok }

// CanAccessProject reports the synthetic authorization decision: a user may
// access a project if they are the owner, a member, or an admin. This is the
// invariant the business-logic detector (CP4) checks violations against.
func (w *World) CanAccessProject(userID, projectID string) bool {
	u, ok := w.Users[userID]
	if !ok {
		return false
	}
	if u.Role == RoleAdmin {
		return true
	}
	p, ok := w.Projects[projectID]
	if !ok {
		return false
	}
	for _, m := range p.MemberIDs {
		if m == userID {
			return true
		}
	}
	return false
}

// AllTokens returns every synthetic API token / secret value in the world, so
// the edge can register them as canaries to watch for exfiltration.
func (w *World) AllTokens() []string {
	var out []string
	for _, u := range w.Users {
		out = append(out, u.APIToken)
	}
	for _, r := range w.Resources {
		if v := r.Data["value"]; v != "" {
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}

func (w *World) sortedUserIDs() []string {
	ids := make([]string, 0, len(w.Users))
	for id := range w.Users {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func (w *World) sortedProjectIDs() []string {
	ids := make([]string, 0, len(w.Projects))
	for id := range w.Projects {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

var nameAdjs = []string{"north", "amber", "quartz", "delta", "orbit", "cedar", "vivid", "lunar"}
var nameNouns = []string{"falcon", "ledger", "atlas", "harbor", "matrix", "pledge", "signal", "vault"}

func synthName(rng *rand.Rand, kind string) string {
	return fmt.Sprintf("%s-%s-%s", kind[:3], nameAdjs[rng.Intn(len(nameAdjs))], nameNouns[rng.Intn(len(nameNouns))])
}

func synthToken(rng *rand.Rand) string {
	const hex = "0123456789abcdef"
	b := make([]byte, 24)
	for i := range b {
		b[i] = hex[rng.Intn(len(hex))]
	}
	return "sk_live_" + string(b) // looks real; is entirely synthetic + watched
}
