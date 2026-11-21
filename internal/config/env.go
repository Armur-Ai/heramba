package config

import "os"

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
	if v := envVar("ALERT_FORMAT", false); v != "" {
		c.Alert.Format = v
	}
	if v := envVar("ALERT_FILE", false); v != "" {
		c.Alert.File = v
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
