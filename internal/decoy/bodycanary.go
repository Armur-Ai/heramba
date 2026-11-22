package decoy

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
)

// BodyCanary is a canary derived for a request-body field: a believable fake
// value to plant (e.g. as the field's "example" in a published API spec) and
// watch for in request bodies. Using it anywhere is, by construction, illegitimate.
type BodyCanary struct {
	Field string `json:"field"` // the request-body field it stands in for
	Token string `json:"token"` // the value to plant and watch
}

// DeriveBodyCanary returns the deterministic canary token for a field under a
// salt. It is deterministic so that the value listed by the CLI (for planting)
// exactly matches the value a running edge watches for. The salt keeps the token
// unguessable: without it, anyone holding the spec could compute and avoid the
// canary.
func DeriveBodyCanary(field, salt string) string {
	sum := sha256.Sum256([]byte(salt + "\x00" + field))
	return canaryPrefix + hex.EncodeToString(sum[:16]) // 128-bit, matches Mint width
}

// BodyCanaries derives a canary for each field (deduplicated, sorted by field).
func BodyCanaries(fields []string, salt string) []BodyCanary {
	seen := map[string]struct{}{}
	uniq := make([]string, 0, len(fields))
	for _, f := range fields {
		if f == "" {
			continue
		}
		if _, ok := seen[f]; ok {
			continue
		}
		seen[f] = struct{}{}
		uniq = append(uniq, f)
	}
	sort.Strings(uniq)
	out := make([]BodyCanary, 0, len(uniq))
	for _, f := range uniq {
		out = append(out, BodyCanary{Field: f, Token: DeriveBodyCanary(f, salt)})
	}
	return out
}

// WatchBodyCanaries registers each derived body canary on the registry's canary
// store so the edge detects it appearing in a request (body/header/query/path).
func (r *Registry) WatchBodyCanaries(cans []BodyCanary) {
	for _, c := range cans {
		r.canaries.Watch(c.Token, "openapi body field: "+c.Field)
	}
}
