package classify

import "testing"

func TestCustomThresholdsRespected(t *testing.T) {
	// A strict operator lowers the agentic threshold so a single tool UA (0.40)
	// is already treated as agentic-suspected.
	c := NewWithOptions(Options{AgenticAt: 0.35})
	v := c.Classify(Observation{Source: "1.1.1.1", Path: "/", UserAgent: "curl/8.1"})
	if v.Label != "agentic-suspected" {
		t.Fatalf("with AgenticAt=0.35 a curl request should be agentic-suspected, got %q (%v)", v.Label, v.Score)
	}
}

func TestZeroOptionsFallBackToDefaults(t *testing.T) {
	c := NewWithOptions(Options{}) // all zero
	if c.opts.CadenceRPS != 5.0 || c.opts.EnumDistinct != 12 || c.opts.ToolAt != 0.4 || c.opts.AgenticAt != 0.7 {
		t.Fatalf("zero options should fall back to defaults, got %+v", c.opts)
	}
}
