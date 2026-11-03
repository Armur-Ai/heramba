// Package bizlogic gives Heramba business-logic understanding: it ingests an
// API description, models object ownership, authorization boundaries, and
// workflow ordering, and detects *sequence-aware* abuse — the multi-request
// authorization violations (IDOR/BOLA, workflow skipping) that signature WAFs
// and per-request classifiers miss.
//
// Findings are explainable and evidence-linked: each names the operation, the
// invariant it broke, and the request sequence that led there. Authorized
// activity that looks structurally identical is allowed — the detector reasons
// about ownership and order, not payloads.
package bizlogic

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Operation is one API operation with its authorization requirements.
type Operation struct {
	ID           string `json:"id"`
	Method       string `json:"method"`
	PathTemplate string `json:"path"` // e.g. /api/projects/{id}
	// RequiredRole is the minimum role (viewer<member<admin) to call it at all.
	RequiredRole string `json:"required_role,omitempty"`
	// OwnershipParam names the path parameter whose referenced object the actor
	// must be authorized to access (object-level authorization / BOLA guard).
	OwnershipParam string `json:"ownership_param,omitempty"`
	// ResourceType is the kind of object OwnershipParam references.
	ResourceType string `json:"resource_type,omitempty"`
}

// Transition is an allowed workflow step: invoking OpID moves to state To.
type Transition struct {
	OpID string `json:"op"`
	To   string `json:"to"`
}

// Workflow is a state machine over operations. Invoking an operation that is
// not an allowed transition from the current state is a sequence violation
// (e.g. confirming an order before a cart exists).
type Workflow struct {
	ID          string                  `json:"id"`
	Start       string                  `json:"start"`
	Transitions map[string][]Transition `json:"transitions"`
}

// Spec is the ingested business-logic model.
type Spec struct {
	Operations []Operation `json:"operations"`
	Workflows  []Workflow  `json:"workflows"`
}

// Match returns the operation whose method+path-template matches a concrete
// request, plus the captured path parameters.
func (s *Spec) Match(method, path string) (Operation, map[string]string, bool) {
	for _, op := range s.Operations {
		if !strings.EqualFold(op.Method, method) {
			continue
		}
		if params, ok := matchTemplate(op.PathTemplate, path); ok {
			return op, params, true
		}
	}
	return Operation{}, nil, false
}

// matchTemplate matches a concrete path against a template with {param}
// segments, returning captured params.
func matchTemplate(tmpl, path string) (map[string]string, bool) {
	ts := splitPath(tmpl)
	ps := splitPath(path)
	if len(ts) != len(ps) {
		return nil, false
	}
	params := map[string]string{}
	for i := range ts {
		if strings.HasPrefix(ts[i], "{") && strings.HasSuffix(ts[i], "}") {
			params[strings.Trim(ts[i], "{}")] = ps[i]
			continue
		}
		if ts[i] != ps[i] {
			return nil, false
		}
	}
	return params, true
}

func splitPath(p string) []string {
	p = strings.Trim(strings.TrimSpace(p), "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}

// LoadSpec parses a native Heramba Spec JSON document.
func LoadSpec(data []byte) (*Spec, error) {
	var s Spec
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("bizlogic: load spec: %w", err)
	}
	return &s, nil
}

// openAPIDoc is the minimal subset of OpenAPI 3 we read, plus x-heramba
// authorization extensions. We parse only what we need (no heavyweight OpenAPI
// dependency), per the mandate to keep dependencies minimal.
type openAPIDoc struct {
	Paths map[string]map[string]openAPIOp `json:"paths"`
}

type openAPIOp struct {
	OperationID     string `json:"operationId"`
	XRequiredRole   string `json:"x-heramba-required-role"`
	XOwnershipParam string `json:"x-heramba-ownership-param"`
	XResourceType   string `json:"x-heramba-resource-type"`
}

// ImportOpenAPI ingests a (subset of) OpenAPI 3 document into a Spec, honoring
// x-heramba-* extensions for authorization metadata. OpenAPI {param} path
// syntax maps directly onto our templates.
func ImportOpenAPI(data []byte) (*Spec, error) {
	var doc openAPIDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("bizlogic: import openapi: %w", err)
	}
	s := &Spec{}
	// Deterministic ordering for stable tests: sort paths then methods.
	for _, path := range sortedKeys(doc.Paths) {
		methods := doc.Paths[path]
		for _, method := range sortedKeys(methods) {
			op := methods[method]
			id := op.OperationID
			if id == "" {
				id = strings.ToLower(method) + ":" + path
			}
			s.Operations = append(s.Operations, Operation{
				ID:             id,
				Method:         strings.ToUpper(method),
				PathTemplate:   path,
				RequiredRole:   op.XRequiredRole,
				OwnershipParam: op.XOwnershipParam,
				ResourceType:   op.XResourceType,
			})
		}
	}
	return s, nil
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	// simple insertion sort to avoid importing sort for one call site pattern
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
