package dashboard

import "sync"

// MemoryController is a safe, in-memory Controller. It records the requested
// state without touching a real edge, so the dashboard is fully functional
// (and testable) standalone. It is the default when no Controller is supplied.
type MemoryController struct {
	mu     sync.RWMutex
	mode   string
	paused bool
	preset string
	clears int
}

// NewMemoryController returns a MemoryController in the default "observe" mode.
func NewMemoryController() *MemoryController {
	return &MemoryController{mode: "observe"}
}

// SetMode records the posture.
func (m *MemoryController) SetMode(mode string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mode = mode
	return nil
}

// Clear bumps the reset counter.
func (m *MemoryController) Clear() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clears++
	return nil
}

// SetPaused records the paused flag.
func (m *MemoryController) SetPaused(paused bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.paused = paused
	return nil
}

// ApplyDecoyPreset records the active preset.
func (m *MemoryController) ApplyDecoyPreset(preset string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.preset = preset
	return nil
}

// State returns a snapshot of the recorded control state (used in tests and the
// control panel's state echo).
func (m *MemoryController) State() (mode string, paused bool, preset string, clears int) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.mode, m.paused, m.preset, m.clears
}
