package taint

import "testing"

// FuzzDecodeObservation ensures the untrusted-input parser never panics and
// never hands attacker-controlled free text to the trusted path.
func FuzzDecodeObservation(f *testing.F) {
	f.Add([]byte(`{"kind":"decoy_hit","source_key":"1.2.3.4","severity":0.5,"nonce":"n","note":"x"}`))
	f.Add([]byte(`{"kind":"pwn"}`))
	f.Add([]byte(`{`))
	f.Add([]byte(``))
	f.Add([]byte(`{"kind":"rate_exceeded","severity":2}`))
	p := Provenance{Sensor: "fuzz", RawRef: "r", Tenant: "t1"}
	f.Fuzz(func(t *testing.T, raw []byte) {
		obs, err := DecodeObservation(raw, p)
		if err != nil {
			return // rejection is fine; it must simply not panic
		}
		// On success, the free text must never be trusted.
		if obs.Note.IsTrusted() {
			t.Fatalf("decoded note must never be trusted: %q", obs.Note.Expose())
		}
		if obs.SourceKey.IsTrusted() {
			t.Fatalf("decoded source key must never be trusted")
		}
		// Severity must be in range when accepted.
		if sev, ok := obs.Severity.Trusted(); ok && (sev < 0 || sev > 1) {
			t.Fatalf("accepted out-of-range severity %v", sev)
		}
	})
}

// FuzzSanitizeForLog ensures log sanitization always strips control characters.
func FuzzSanitizeForLog(f *testing.F) {
	f.Add("hello\nworld")
	f.Add("\x1b[2Jboom")
	f.Fuzz(func(t *testing.T, s string) {
		out := SanitizeForLog(s)
		for _, r := range out {
			if r == '\n' || r == '\r' || r == '\x1b' {
				t.Fatalf("sanitized string still contains a control rune %q", out)
			}
		}
	})
}
