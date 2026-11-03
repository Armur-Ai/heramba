package decoy

import "testing"

// FuzzDecoyRouting ensures decoy routing never panics on arbitrary method/path
// input (attacker-controlled request lines).
func FuzzDecoyRouting(f *testing.F) {
	f.Add("GET", "/.env")
	f.Add("POST", "/%%%/../../etc")
	f.Add("", "")
	f.Add("GET", string([]byte{0x00, 0x7f, 0x1b}))
	reg := NewRegistry("/.env", "/.git/config")
	f.Fuzz(func(t *testing.T, method, path string) {
		_, _ = reg.IsDecoy(path)
		_, _ = reg.InspectPath(method, path, "203.0.113.9")
	})
}
