package config

import "testing"

func TestApplyEnvOverlaysPrefixed(t *testing.T) {
	t.Setenv("HERAMBA_MODE", "protect")
	t.Setenv("HERAMBA_UPSTREAM", "http://app:3000")
	t.Setenv("HERAMBA_LISTEN", ":9090")
	t.Setenv("HERAMBA_ALERT_FORMAT", "jsonl")
	t.Setenv("HERAMBA_ALERT_FILE", "/var/log/heramba.jsonl")

	c := Default().ApplyEnv()
	if c.Mode != ModeProtect || c.Upstream != "http://app:3000" || c.Listen != ":9090" {
		t.Fatalf("prefixed env not applied: %+v", c)
	}
	if c.Alert.Format != "jsonl" || c.Alert.File != "/var/log/heramba.jsonl" {
		t.Fatalf("alert env not applied: %+v", c.Alert)
	}
}

func TestApplyEnvBareOnlyForModeAndUpstream(t *testing.T) {
	t.Setenv("MODE", "protect")
	t.Setenv("UPSTREAM", "http://app:3000")
	t.Setenv("LISTEN", ":1234") // bare LISTEN must be ignored (collision-prone)

	c := Default().ApplyEnv()
	if c.Mode != ModeProtect || c.Upstream != "http://app:3000" {
		t.Fatalf("bare MODE/UPSTREAM should apply: %+v", c)
	}
	if c.Listen == ":1234" {
		t.Fatal("bare LISTEN must NOT be honored (prefix required)")
	}
}

func TestApplyEnvSIEMEndpoints(t *testing.T) {
	t.Setenv("HERAMBA_SPLUNK_URL", "https://hec:8088/services/collector/event")
	t.Setenv("HERAMBA_SPLUNK_TOKEN", "tok")
	t.Setenv("HERAMBA_DATADOG_API_KEY", "ddk")
	t.Setenv("HERAMBA_ELASTIC_URL", "https://es:9200")
	t.Setenv("HERAMBA_SYSLOG_ADDRESS", "siem:514")
	c := Default().ApplyEnv()
	if c.Alert.Splunk.URL == "" || c.Alert.Splunk.Token != "tok" {
		t.Errorf("splunk env not applied: %+v", c.Alert.Splunk)
	}
	if c.Alert.Datadog.APIKey != "ddk" || c.Alert.Elastic.URL == "" || c.Alert.Syslog.Address != "siem:514" {
		t.Errorf("siem env not applied: %+v", c.Alert)
	}
}

func TestApplyEnvPrefixWinsOverBare(t *testing.T) {
	t.Setenv("UPSTREAM", "http://bare:1")
	t.Setenv("HERAMBA_UPSTREAM", "http://prefixed:2")
	if got := Default().ApplyEnv().Upstream; got != "http://prefixed:2" {
		t.Fatalf("prefix must win, got %q", got)
	}
}

func TestApplyEnvNoVarsIsNoOp(t *testing.T) {
	// Ensure a clean environment for the keys we read.
	for _, k := range []string{"HERAMBA_MODE", "HERAMBA_UPSTREAM", "HERAMBA_LISTEN", "HERAMBA_ALERT_FORMAT", "HERAMBA_ALERT_FILE", "HERAMBA_WEBHOOK", "HERAMBA_SLACK", "HERAMBA_ALERT_MIN_SEVERITY", "MODE", "UPSTREAM"} {
		t.Setenv(k, "")
	}
	base := Default()
	got := base.ApplyEnv()
	if got.Mode != base.Mode || got.Upstream != base.Upstream || got.Listen != base.Listen ||
		got.Alert != base.Alert {
		t.Fatalf("empty env should be a no-op: %+v != %+v", got, base)
	}
}
