package evidence

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"sync"
	"time"
)

// RecordType tags a persisted line so a mixed evidence log can be read back
// into the right concrete type.
type RecordType string

const (
	TypeSecurityEvent RecordType = "security_event"
	TypeFact          RecordType = "fact"
	TypeHypothesis    RecordType = "hypothesis"
	TypeRelationship  RecordType = "relationship"
)

// envelope is the on-disk JSONL line format: a type tag plus the raw record.
type envelope struct {
	Schema int             `json:"schema"`
	Type   RecordType      `json:"type"`
	Record json.RawMessage `json:"record"`
}

// Store is an in-memory, concurrency-safe evidence store with optional
// append-only JSONL durability. Every Put validates before it accepts, so the
// store never holds (or persists) a malformed record. It is deliberately simple
// — a local file and a few maps — per the mandate to avoid heavy dependencies
// until measurements justify them.
type Store struct {
	mu     sync.RWMutex
	events map[ID]SecurityEvent
	facts  map[ID]Fact
	hyps   map[ID]*Hypothesis
	rels   map[ID]Relationship

	w  io.Writer // append sink (nil = in-memory only)
	bw *bufio.Writer
	f  *os.File
}

// NewStore returns an in-memory store with no persistence.
func NewStore() *Store {
	return &Store{
		events: map[ID]SecurityEvent{},
		facts:  map[ID]Fact{},
		hyps:   map[ID]*Hypothesis{},
		rels:   map[ID]Relationship{},
	}
}

// OpenStore opens (creating if needed) an append-only JSONL evidence log at
// path, loads any existing records, and returns a store that appends new Puts
// to it. Existing records with an unknown schema are skipped with a count, not
// a crash — forward/backward compatibility is explicit.
func OpenStore(path string) (*Store, int, error) {
	s := NewStore()
	skipped := 0
	if data, err := os.ReadFile(path); err == nil {
		skipped, err = s.loadBytes(data)
		if err != nil {
			return nil, skipped, err
		}
	} else if !os.IsNotExist(err) {
		return nil, 0, fmt.Errorf("read evidence log: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, skipped, fmt.Errorf("open evidence log: %w", err)
	}
	s.f = f
	s.bw = bufio.NewWriter(f)
	s.w = s.bw
	return s, skipped, nil
}

// Close flushes and closes the durability log, if any.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.bw != nil {
		if err := s.bw.Flush(); err != nil {
			return err
		}
	}
	if s.f != nil {
		return s.f.Close()
	}
	return nil
}

func (s *Store) append(t RecordType, rec any) error {
	if s.w == nil {
		return nil
	}
	raw, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	line, err := json.Marshal(envelope{Schema: SchemaVersion, Type: t, Record: raw})
	if err != nil {
		return err
	}
	if _, err := s.w.Write(append(line, '\n')); err != nil {
		return err
	}
	if s.bw != nil {
		return s.bw.Flush()
	}
	return nil
}

// PutEvent validates and stores a SecurityEvent (and persists it if durable).
func (s *Store) PutEvent(e SecurityEvent) error {
	if err := e.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events[e.ID] = e
	return s.append(TypeSecurityEvent, e)
}

// PutFact validates and stores a Fact.
func (s *Store) PutFact(f Fact) error {
	if err := f.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.facts[f.ID] = f
	return s.append(TypeFact, f)
}

// PutHypothesis validates and stores a Hypothesis.
func (s *Store) PutHypothesis(h *Hypothesis) error {
	if err := h.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hyps[h.ID] = h
	return s.append(TypeHypothesis, h)
}

// PutRelationship validates and stores a Relationship.
func (s *Store) PutRelationship(r Relationship) error {
	if err := r.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rels[r.ID] = r
	return s.append(TypeRelationship, r)
}

// Event returns a stored SecurityEvent by ID.
func (s *Store) Event(id ID) (SecurityEvent, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.events[id]
	return e, ok
}

// Hypothesis returns a stored Hypothesis by ID.
func (s *Store) Hypothesis(id ID) (*Hypothesis, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	h, ok := s.hyps[id]
	return h, ok
}

// Counts reports how many of each record type the store holds.
func (s *Store) Counts() (events, facts, hyps, rels int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.events), len(s.facts), len(s.hyps), len(s.rels)
}

// EventsBySource returns every SecurityEvent whose transport source key matches,
// sorted chronologically by observation time. This is a *transport* grouping,
// not an identity claim.
func (s *Store) EventsBySource(key string) []SecurityEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []SecurityEvent
	for _, e := range s.events {
		if e.SourceKey == key {
			out = append(out, e)
		}
	}
	sortEventsByTime(out)
	return out
}

// EventsInWindow returns every SecurityEvent observed in [from, to], sorted
// chronologically. A zero `to` means "up to now".
func (s *Store) EventsInWindow(from, to time.Time) []SecurityEvent {
	if to.IsZero() {
		to = time.Now().UTC()
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []SecurityEvent
	for _, e := range s.events {
		t := e.Provenance.ObservedAt
		if !t.Before(from) && !t.After(to) {
			out = append(out, e)
		}
	}
	sortEventsByTime(out)
	return out
}

// RelationshipsFrom returns edges originating at a given record ID.
func (s *Store) RelationshipsFrom(id ID) []Relationship {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Relationship
	for _, r := range s.rels {
		if r.From == id {
			out = append(out, r)
		}
	}
	return out
}

func sortEventsByTime(es []SecurityEvent) {
	sort.SliceStable(es, func(i, j int) bool {
		return es[i].Provenance.ObservedAt.Before(es[j].Provenance.ObservedAt)
	})
}

// loadBytes replays a JSONL evidence log into the store. It returns the number
// of records skipped for an unknown schema. A malformed line is a hard error
// (the log is corrupt); a known-type line with a wrong schema is a soft skip.
func (s *Store) loadBytes(data []byte) (int, error) {
	skipped := 0
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	line := 0
	for sc.Scan() {
		line++
		raw := sc.Bytes()
		if len(raw) == 0 {
			continue
		}
		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return skipped, fmt.Errorf("evidence log line %d: %w", line, err)
		}
		if env.Schema != SchemaVersion {
			skipped++
			continue
		}
		if err := s.loadRecord(env); err != nil {
			return skipped, fmt.Errorf("evidence log line %d: %w", line, err)
		}
	}
	return skipped, sc.Err()
}

func (s *Store) loadRecord(env envelope) error {
	switch env.Type {
	case TypeSecurityEvent:
		var e SecurityEvent
		if err := json.Unmarshal(env.Record, &e); err != nil {
			return err
		}
		if err := e.Validate(); err != nil {
			return err
		}
		s.events[e.ID] = e
	case TypeFact:
		var f Fact
		if err := json.Unmarshal(env.Record, &f); err != nil {
			return err
		}
		if err := f.Validate(); err != nil {
			return err
		}
		s.facts[f.ID] = f
	case TypeHypothesis:
		var h Hypothesis
		if err := json.Unmarshal(env.Record, &h); err != nil {
			return err
		}
		if err := h.Validate(); err != nil {
			return err
		}
		s.hyps[h.ID] = &h
	case TypeRelationship:
		var r Relationship
		if err := json.Unmarshal(env.Record, &r); err != nil {
			return err
		}
		if err := r.Validate(); err != nil {
			return err
		}
		s.rels[r.ID] = r
	default:
		return invalid("unknown record type %q", env.Type)
	}
	return nil
}
