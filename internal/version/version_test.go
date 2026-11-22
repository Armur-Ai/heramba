package version

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestInfoCarriesBuildVars(t *testing.T) {
	i := Info()
	if i.Version != Version || i.Commit != Commit || i.Date != Date {
		t.Errorf("Info() must mirror build vars: %+v", i)
	}
	if !strings.HasPrefix(i.Go, "go") {
		t.Errorf("Go runtime should be reported, got %q", i.Go)
	}
}

func TestInfoJSONShape(t *testing.T) {
	b, err := json.Marshal(Info())
	if err != nil {
		t.Fatal(err)
	}
	var back map[string]any
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"version", "commit", "date", "go"} {
		if _, ok := back[key]; !ok {
			t.Errorf("version JSON missing key %q: %s", key, b)
		}
	}
}
