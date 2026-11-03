package evidence

import (
	"os"
	"path/filepath"
	"testing"
)

// FuzzStoreLoad ensures the append-only JSONL loader never panics on a corrupt
// or hostile log line (it must validate-before-accept and soft-skip unknowns).
func FuzzStoreLoad(f *testing.F) {
	f.Add([]byte(`{"schema_version":1,"type":"event"}`))
	f.Add([]byte(`{"schema_version":999,"type":"event"}`)) // unknown schema -> soft-skip
	f.Add([]byte("not json\n{bad"))
	f.Add([]byte(``))
	f.Fuzz(func(t *testing.T, data []byte) {
		dir := t.TempDir()
		path := filepath.Join(dir, "log.jsonl")
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Skip()
		}
		// OpenStore reads and validates; a corrupt line must not panic.
		s, _, err := OpenStore(path)
		if err == nil && s != nil {
			_ = s.Close()
		}
	})
}
