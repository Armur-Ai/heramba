package alert

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/Armur-Ai/heramba/internal/event"
)

// RotatingFileSink writes each event as one JSON line to a file, rotating the
// file once it would exceed maxBytes. Rotation renames the active file to
// "<path>.1", shifting any existing "<path>.N" up to "<path>.N+1" and dropping
// anything past maxBackups. This gives an operator a durable, bounded on-disk
// audit trail with no external dependency and no network — the local-sink
// counterpart to the SIEM push integrations.
//
// Safe for concurrent use (the edge emits from many goroutines).
type RotatingFileSink struct {
	mu         sync.Mutex
	path       string
	maxBytes   int64
	maxBackups int
	f          *os.File
	size       int64
}

// NewRotatingFile opens (creating/appending) the file at path. maxBytes is the
// size at which the file rotates (<=0 defaults to 10 MiB); maxBackups is how
// many rotated files to keep (<0 clamps to 0). It returns an error if the file
// cannot be opened.
func NewRotatingFile(path string, maxBytes int64, maxBackups int) (*RotatingFileSink, error) {
	if maxBytes <= 0 {
		maxBytes = 10 << 20
	}
	if maxBackups < 0 {
		maxBackups = 0
	}
	s := &RotatingFileSink{path: path, maxBytes: maxBytes, maxBackups: maxBackups}
	if err := s.open(); err != nil {
		return nil, err
	}
	return s, nil
}

// open opens the active file for append and records its current size.
func (s *RotatingFileSink) open() error {
	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644) //nolint:gosec
	if err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return err
	}
	s.f = f
	s.size = info.Size()
	return nil
}

// Emit appends the event as a JSON line, rotating first if the write would push
// the file past maxBytes.
func (s *RotatingFileSink) Emit(e event.Event) error {
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	line := append(b, '\n')

	s.mu.Lock()
	defer s.mu.Unlock()
	// Rotate before writing when the file is non-empty and this line would
	// exceed the cap, so a single oversized line still lands in a fresh file
	// rather than being dropped.
	if s.size > 0 && s.size+int64(len(line)) > s.maxBytes {
		if err := s.rotate(); err != nil {
			return err
		}
	}
	n, err := s.f.Write(line)
	s.size += int64(n)
	return err
}

// rotate closes the active file, shifts the backup chain, and reopens a fresh
// active file. The caller must hold s.mu.
func (s *RotatingFileSink) rotate() error {
	if err := s.f.Close(); err != nil {
		return err
	}
	if s.maxBackups == 0 {
		// No backups kept: just truncate by reopening with O_TRUNC.
		f, err := os.OpenFile(s.path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644) //nolint:gosec
		if err != nil {
			return err
		}
		s.f = f
		s.size = 0
		return nil
	}
	// Drop the oldest, then shift N-1->N, …, 1->2, active->1.
	_ = os.Remove(backupName(s.path, s.maxBackups))
	for i := s.maxBackups - 1; i >= 1; i-- {
		older := backupName(s.path, i)
		if _, err := os.Stat(older); err == nil {
			_ = os.Rename(older, backupName(s.path, i+1))
		}
	}
	if err := os.Rename(s.path, backupName(s.path, 1)); err != nil {
		return err
	}
	return s.open()
}

// Close closes the active file.
func (s *RotatingFileSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.f == nil {
		return nil
	}
	err := s.f.Close()
	s.f = nil
	return err
}

// backupName returns the rotated filename for index i, kept in the same
// directory so rotation never crosses a filesystem boundary.
func backupName(path string, i int) string {
	return filepath.Clean(fmt.Sprintf("%s.%d", path, i))
}
