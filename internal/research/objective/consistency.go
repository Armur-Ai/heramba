// Package objective is the R3 deception & attack-objective experiment. It has
// two halves:
//
//   - A *real* consistency validator and isolation proofs that run against the
//     actual deception.World and assetgraph packages: synthetic resources must
//     be internally consistent (referential integrity, coherent permissions and
//     workflows, reproducible temporal state, structural synthetic tags) and
//     must be unable to leak into or act on a real system.
//
//   - A transparent, seeded *model* of five defensive arms (unprotected,
//     conventional rate limiting, static decoys, stateful deception, adaptive
//     deception) measuring attacker objective completion and diversion — tested
//     against a deception-aware adversary that rejects inconsistent lures.
//
// The model is clearly labeled a model; the validator and isolation checks are
// real code with real tests, because those are the safety-critical claims.
package objective

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Armur-Ai/heramba/internal/deception"
)

// Consistency validation errors.
var (
	ErrNotSynthetic    = errors.New("consistency: record is not tagged synthetic")
	ErrDanglingRef     = errors.New("consistency: reference to a non-existent record")
	ErrPermIncoherent  = errors.New("consistency: incoherent permission (owner not a member)")
	ErrRealEmailDomain = errors.New("consistency: synthetic email uses a non-reserved (possibly real) domain")
	ErrZeroTemporal    = errors.New("consistency: temporal state is zero/unset")
	ErrDuplicateID     = errors.New("consistency: duplicate identifier")
	ErrEmptyWorld      = errors.New("consistency: world has no users or projects")
)

// reservedEmailSuffixes are the only domains a synthetic user may use. They are
// reserved by RFC 2606 / 6761 and can never resolve to a real system.
var reservedEmailSuffixes = []string{".test", ".example", ".invalid", ".localhost",
	"@example.com", "@example.net", "@example.org", "example-corp.test"}

// ValidateWorld checks a synthetic world for internal consistency across the
// dimensions R3 requires: identifiers, permissions, workflows, temporal state,
// and the synthetic tag. A deception-aware adversary probes exactly these; an
// inconsistency is what gives a lure away, so the validator is the defender's
// own pre-flight check that the world holds together. Returns a joined error.
func ValidateWorld(w *deception.World) error {
	if w == nil || len(w.Users) == 0 || len(w.Projects) == 0 {
		return ErrEmptyWorld
	}
	var errs []error

	// Temporal state must be deterministic and set (never the zero time).
	if w.CreatedAt.IsZero() {
		errs = append(errs, ErrZeroTemporal)
	}

	// Identifiers: unique and structurally tagged synthetic.
	seen := map[string]bool{}
	checkID := func(id, kind string) {
		if id == "" {
			errs = append(errs, fmt.Errorf("%w: empty %s id", ErrDanglingRef, kind))
			return
		}
		if seen[id] {
			errs = append(errs, fmt.Errorf("%w: %s", ErrDuplicateID, id))
		}
		seen[id] = true
	}

	for id, u := range w.Users {
		checkID(id, "user")
		if !u.Synthetic {
			errs = append(errs, fmt.Errorf("%w: user %s", ErrNotSynthetic, id))
		}
		if !hasReservedSuffix(u.Email) {
			errs = append(errs, fmt.Errorf("%w: user %s email %q", ErrRealEmailDomain, id, u.Email))
		}
	}
	for id, p := range w.Projects {
		checkID(id, "project")
		if !p.Synthetic {
			errs = append(errs, fmt.Errorf("%w: project %s", ErrNotSynthetic, id))
		}
		// Workflow / referential integrity: owner and members must exist.
		if _, ok := w.Users[p.OwnerID]; !ok {
			errs = append(errs, fmt.Errorf("%w: project %s owner %s", ErrDanglingRef, id, p.OwnerID))
		}
		ownerIsMember := false
		for _, m := range p.MemberIDs {
			if _, ok := w.Users[m]; !ok {
				errs = append(errs, fmt.Errorf("%w: project %s member %s", ErrDanglingRef, id, m))
			}
			if m == p.OwnerID {
				ownerIsMember = true
			}
		}
		// Permission coherence: the owner must be able to access their own project.
		if !ownerIsMember && !w.CanAccessProject(p.OwnerID, id) {
			errs = append(errs, fmt.Errorf("%w: project %s", ErrPermIncoherent, id))
		}
	}
	for id, r := range w.Resources {
		checkID(id, "resource")
		if !r.Synthetic {
			errs = append(errs, fmt.Errorf("%w: resource %s", ErrNotSynthetic, id))
		}
		if _, ok := w.Projects[r.ProjectID]; !ok {
			errs = append(errs, fmt.Errorf("%w: resource %s project %s", ErrDanglingRef, id, r.ProjectID))
		}
	}

	return errors.Join(errs...)
}

func hasReservedSuffix(email string) bool {
	e := strings.ToLower(email)
	for _, s := range reservedEmailSuffixes {
		if strings.Contains(e, s) {
			return true
		}
	}
	return false
}

// Reproducible asserts a world regenerated from the same seed is byte-identical
// in structure (temporal state, counts, ids, tokens) — a deception-aware
// adversary that observes two snapshots must not catch drift.
func Reproducible(seed int64, nUsers, nProjects, nResources int) error {
	a := deception.GenerateWorld(seed, nUsers, nProjects, nResources)
	b := deception.GenerateWorld(seed, nUsers, nProjects, nResources)
	if !a.CreatedAt.Equal(b.CreatedAt) {
		return fmt.Errorf("temporal state differs: %v vs %v", a.CreatedAt, b.CreatedAt)
	}
	if len(a.Users) != len(b.Users) || len(a.Projects) != len(b.Projects) || len(a.Resources) != len(b.Resources) {
		return errors.New("record counts differ across regeneration")
	}
	for id, ua := range a.Users {
		ub, ok := b.Users[id]
		if !ok || ua.APIToken != ub.APIToken || ua.Email != ub.Email || ua.Role != ub.Role {
			return fmt.Errorf("user %s differs across regeneration", id)
		}
	}
	return nil
}
