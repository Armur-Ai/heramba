package config

import (
	"os"
	"strings"
)

// splitList parses a comma-separated env value into a trimmed, non-empty list.
func splitList(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ApplyEnv overlays HERAMBA_* environment variables onto the config and returns
// the result. This is what makes the one-container deploy work with no config
// file at all:
//
//	docker run -e HERAMBA_UPSTREAM=http://app:3000 -e HERAMBA_MODE=observe armur/heramba
//
// Precedence is flags > env > file > defaults: callers apply this after loading
// the file and before applying explicit CLI flags, so an operator can bake a
// base config into an image and tune each deployment purely with env vars.
//
// As a convenience for the headline one-liner, the two most common knobs —
// MODE and UPSTREAM — are also read from the bare (unprefixed) name; the
// HERAMBA_-prefixed form always wins. Everything else requires the prefix to
// avoid colliding with unrelated environment variables.
func (c Config) ApplyEnv() Config {
	if v := envVar("MODE", true); v != "" {
		c.Mode = Mode(v)
	}
	if v := envVar("UPSTREAM", true); v != "" {
		c.Upstream = v
	}
	if v := envVar("LISTEN", false); v != "" {
		c.Listen = v
	}
	if v := envVar("ADMIN_LISTEN", false); v != "" {
		c.AdminListen = v
	}
	if v := envVar("ALERT_FORMAT", false); v != "" {
		c.Alert.Format = v
	}
	if v := envVar("ALERT_FILE", false); v != "" {
		c.Alert.File = v
	}
	if v := envVar("WEBHOOK", false); v != "" {
		c.Alert.Webhook = v
	}
	if v := envVar("SLACK", false); v != "" {
		c.Alert.Slack = v
	}
	if v := envVar("TEAMS", false); v != "" {
		c.Alert.Teams = v
	}
	if v := envVar("DISCORD", false); v != "" {
		c.Alert.Discord = v
	}
	if v := envVar("GOOGLE_CHAT", false); v != "" {
		c.Alert.GoogleChat = v
	}
	if v := envVar("MATTERMOST", false); v != "" {
		c.Alert.Mattermost = v
	}
	if v := envVar("ROTATE_FILE", false); v != "" {
		c.Alert.Rotate.File = v
	}
	if v := envVar("EMAIL_HOST", false); v != "" {
		c.Alert.Email.Host = v
	}
	if v := envVar("EMAIL_FROM", false); v != "" {
		c.Alert.Email.From = v
	}
	if v := envVar("EMAIL_TO", false); v != "" {
		c.Alert.Email.To = splitList(v)
	}
	if v := envVar("ALERT_MIN_SEVERITY", false); v != "" {
		c.Alert.MinSeverity = v
	}
	if v := envVar("PRESET", false); v != "" {
		c.Decoys.Preset = v
	}
	// SIEM endpoints
	if v := envVar("SPLUNK_URL", false); v != "" {
		c.Alert.Splunk.URL = v
	}
	if v := envVar("SPLUNK_TOKEN", false); v != "" {
		c.Alert.Splunk.Token = v
	}
	if v := envVar("DATADOG_API_KEY", false); v != "" {
		c.Alert.Datadog.APIKey = v
	}
	if v := envVar("ELASTIC_URL", false); v != "" {
		c.Alert.Elastic.URL = v
	}
	if v := envVar("SYSLOG_ADDRESS", false); v != "" {
		c.Alert.Syslog.Address = v
	}
	if v := envVar("LOKI_URL", false); v != "" {
		c.Alert.Loki.URL = v
	}
	if v := envVar("NEWRELIC_API_KEY", false); v != "" {
		c.Alert.NewRelic.APIKey = v
	}
	if v := envVar("HONEYCOMB_API_KEY", false); v != "" {
		c.Alert.Honeycomb.APIKey = v
	}
	if v := envVar("HONEYCOMB_DATASET", false); v != "" {
		c.Alert.Honeycomb.Dataset = v
	}
	if v := envVar("OPENOBSERVE_URL", false); v != "" {
		c.Alert.OpenObserve.URL = v
	}
	if v := envVar("AXIOM_TOKEN", false); v != "" {
		c.Alert.Axiom.Token = v
	}
	if v := envVar("AXIOM_DATASET", false); v != "" {
		c.Alert.Axiom.Dataset = v
	}
	if v := envVar("SUMO_URL", false); v != "" {
		c.Alert.Sumo = v
	}
	if v := envVar("PAGERDUTY_KEY", false); v != "" {
		c.Alert.PagerDutyKey = v
	}
	if v := envVar("OPSGENIE_KEY", false); v != "" {
		c.Alert.OpsgenieKey = v
	}
	return c
}

// envVar reads HERAMBA_<name>, falling back to the bare <name> only when
// allowBare is set.
func envVar(name string, allowBare bool) string {
	if v := os.Getenv("HERAMBA_" + name); v != "" {
		return v
	}
	if allowBare {
		return os.Getenv(name)
	}
	return ""
}
