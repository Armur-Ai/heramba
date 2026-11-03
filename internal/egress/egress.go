// Package egress is a callback/egress firewall. In PROTECT mode it enforces a
// deny-by-default allowlist on outbound connections the protected app makes, so
// an injected payload cannot phone home, exfiltrate, or fetch an out-of-band
// callback (the backbone of SSRF/OOB and agent data-exfil). A blocked attempt
// is a high-signal detection event.
package egress

import (
	"context"
	"fmt"
	"net"

	"github.com/Armur-Ai/heramba/internal/alert"
	"github.com/Armur-Ai/heramba/internal/event"
)

// DialFunc is the signature of a context-aware dialer (matches net.Dialer.DialContext).
type DialFunc func(ctx context.Context, network, addr string) (net.Conn, error)

// Firewall enforces an egress allowlist and reports blocks.
type Firewall struct {
	enabled bool
	allowed map[string]bool
	sink    alert.Sink
	base    DialFunc
}

// New builds a firewall. When enabled is false it is a pass-through (observe
// mode never blocks). base is the underlying dialer (nil => net.Dialer).
func New(enabled bool, allowedHosts []string, sink alert.Sink, base DialFunc) *Firewall {
	set := make(map[string]bool, len(allowedHosts))
	for _, h := range allowedHosts {
		if h != "" {
			set[h] = true
		}
	}
	if sink == nil {
		sink = alert.Discard
	}
	if base == nil {
		base = (&net.Dialer{}).DialContext
	}
	return &Firewall{enabled: enabled, allowed: set, sink: sink, base: base}
}

// Allowed reports whether a host may be dialed. When the firewall is disabled,
// everything is allowed.
func (f *Firewall) Allowed(host string) bool {
	if !f.enabled {
		return true
	}
	return f.allowed[host]
}

// BlockedError is returned when a dial is denied by policy.
type BlockedError struct{ Host string }

func (e *BlockedError) Error() string {
	return fmt.Sprintf("egress: host %q is not on the allowlist", e.Host)
}

// DialContext enforces the allowlist, emits an event on a block, and otherwise
// delegates to the base dialer.
func (f *Firewall) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host := addr
	if h, _, err := net.SplitHostPort(addr); err == nil {
		host = h
	}
	if f.enabled && !f.Allowed(host) {
		_ = f.sink.Emit(event.New(event.KindEgressBlocked, event.SeverityHigh, "app", addr,
			"blocked out-of-policy outbound connection").WithLabel("host", host))
		return nil, &BlockedError{Host: host}
	}
	return f.base(ctx, network, addr)
}
