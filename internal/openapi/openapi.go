// Package openapi extracts request-body field names from an OpenAPI/Swagger
// spec. Heramba uses it to seed request-body canaries: a believable fake value
// is derived for each sensitive-looking body field, so if an attacker who
// scraped the API spec ever submits one of those planted values, it is a
// zero-false-positive intrusion signal (nobody legitimate carries it).
//
// The parser is deliberately dependency-free (encoding/json only) and best-
// effort: it walks OpenAPI v3 requestBody schemas and Swagger v2 body/formData
// parameters, resolving local $ref into components.schemas / definitions. A
// malformed or unfamiliar spec yields fewer fields, never a crash.
package openapi

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// sensitiveHints are substrings that mark a field name as worth planting a
// canary for. Matching is case-insensitive on the field name.
var sensitiveHints = []string{
	"token", "secret", "password", "passwd", "pwd", "apikey", "api_key",
	"key", "auth", "credential", "session", "cookie", "ssn", "email",
	"card", "cvv", "pin", "private", "access", "bearer",
}

// IsSensitive reports whether a field name looks sensitive enough to seed a
// canary for.
func IsSensitive(name string) bool {
	l := strings.ToLower(name)
	for _, h := range sensitiveHints {
		if strings.Contains(l, h) {
			return true
		}
	}
	return false
}

// SensitiveFields parses an OpenAPI/Swagger JSON spec and returns the sorted,
// unique, sensitive-looking request-body field names found in it.
func SensitiveFields(spec []byte) ([]string, error) {
	all, err := BodyFields(spec)
	if err != nil {
		return nil, err
	}
	out := all[:0:0]
	for _, f := range all {
		if IsSensitive(f) {
			out = append(out, f)
		}
	}
	return out, nil
}

// BodyFields parses the spec and returns every (sorted, unique) request-body
// field name it finds, unfiltered.
func BodyFields(spec []byte) ([]string, error) {
	var doc map[string]any
	if err := json.Unmarshal(spec, &doc); err != nil {
		return nil, fmt.Errorf("openapi: parse spec: %w", err)
	}
	w := &walker{doc: doc, found: map[string]struct{}{}, seenRef: map[string]bool{}}

	paths, _ := doc["paths"].(map[string]any)
	for _, pi := range paths {
		item, ok := pi.(map[string]any)
		if !ok {
			continue
		}
		for _, method := range []string{"get", "put", "post", "delete", "patch", "options", "head", "trace"} {
			op, ok := item[method].(map[string]any)
			if !ok {
				continue
			}
			// OpenAPI v3: requestBody.content.<mediatype>.schema
			if rb, ok := op["requestBody"].(map[string]any); ok {
				if content, ok := rb["content"].(map[string]any); ok {
					for _, mt := range content {
						if m, ok := mt.(map[string]any); ok {
							w.walkSchema(m["schema"])
						}
					}
				}
			}
			// Swagger v2: parameters[] with in: body/formData.
			if params, ok := op["parameters"].([]any); ok {
				for _, p := range params {
					pm, ok := p.(map[string]any)
					if !ok {
						continue
					}
					switch pm["in"] {
					case "body":
						w.walkSchema(pm["schema"])
					case "formData":
						if name, ok := pm["name"].(string); ok {
							w.add(name)
						}
					}
				}
			}
		}
	}

	res := make([]string, 0, len(w.found))
	for f := range w.found {
		res = append(res, f)
	}
	sort.Strings(res)
	return res, nil
}

type walker struct {
	doc     map[string]any
	found   map[string]struct{}
	seenRef map[string]bool
}

func (w *walker) add(name string) {
	if name != "" {
		w.found[name] = struct{}{}
	}
}

// walkSchema descends a schema node collecting property names, resolving local
// $ref and recursing through properties, array items, and allOf/anyOf/oneOf.
func (w *walker) walkSchema(node any) {
	s, ok := node.(map[string]any)
	if !ok {
		return
	}
	if ref, ok := s["$ref"].(string); ok {
		w.walkRef(ref)
		return
	}
	if props, ok := s["properties"].(map[string]any); ok {
		for name, child := range props {
			w.add(name)
			w.walkSchema(child)
		}
	}
	if items, ok := s["items"]; ok {
		w.walkSchema(items)
	}
	for _, comb := range []string{"allOf", "anyOf", "oneOf"} {
		if list, ok := s[comb].([]any); ok {
			for _, c := range list {
				w.walkSchema(c)
			}
		}
	}
}

// walkRef resolves a local reference (#/components/schemas/X or
// #/definitions/X) and walks the target schema once.
func (w *walker) walkRef(ref string) {
	if w.seenRef[ref] {
		return // cycle guard
	}
	w.seenRef[ref] = true
	const prefix = "#/"
	if !strings.HasPrefix(ref, prefix) {
		return
	}
	cur := any(w.doc)
	for _, seg := range strings.Split(ref[len(prefix):], "/") {
		m, ok := cur.(map[string]any)
		if !ok {
			return
		}
		cur = m[seg]
	}
	w.walkSchema(cur)
}
