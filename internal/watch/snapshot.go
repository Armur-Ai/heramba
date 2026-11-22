package watch

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Snapshot is a point-in-time, serializable summary of the dashboard state —
// what an operator exports with the 's' key to drop into a ticket or a chat.
type Snapshot struct {
	GeneratedAt time.Time        `json:"generated_at"`
	Mode        string           `json:"mode"`
	UptimeSecs  int              `json:"uptime_seconds"`
	ThreatLevel string           `json:"threat_level"`
	ThreatScore float64          `json:"threat_score"`
	Total       int              `json:"total_events"`
	Sources     int              `json:"distinct_sources"`
	Swarms      int              `json:"swarms"`
	ByKind      map[string]int   `json:"by_kind"`
	BySeverity  map[string]int   `json:"by_severity"`
	TopSources  []SnapshotSource `json:"top_sources"`
	TopDecoys   []SnapshotDecoy  `json:"top_decoys"`
}

// SnapshotSource is one row of the exported top-sources table.
type SnapshotSource struct {
	Addr     string `json:"addr"`
	Severity string `json:"severity"`
	Hits     int    `json:"hits"`
	Decoys   int    `json:"decoys"`
	Canaries int    `json:"canaries"`
}

// SnapshotDecoy is one row of the exported decoy-hit table.
type SnapshotDecoy struct {
	Target string `json:"target"`
	Hits   int    `json:"hits"`
}

// Snapshot builds a summary of the current state as of `now`, labeled with the
// posture mode and the time since `started`.
func (s *State) Snapshot(mode string, started, now time.Time) Snapshot {
	snap := Snapshot{
		GeneratedAt: now.UTC(),
		Mode:        mode,
		UptimeSecs:  int(now.Sub(started).Seconds()),
		ThreatLevel: s.ThreatLevel().String(),
		ThreatScore: s.ThreatScore(),
		Total:       s.Total,
		Sources:     s.SourceCount(),
		Swarms:      s.SwarmCount(),
		ByKind:      map[string]int{},
		BySeverity:  map[string]int{},
	}
	for k, v := range s.ByKind {
		snap.ByKind[string(k)] = v
	}
	for k, v := range s.BySeverity {
		snap.BySeverity[k.String()] = v
	}
	for _, src := range s.TopSources(10) {
		snap.TopSources = append(snap.TopSources, SnapshotSource{
			Addr: src.Addr, Severity: src.MaxSeverity.String(),
			Hits: src.Count, Decoys: src.Decoys, Canaries: src.Canaries,
		})
	}
	for _, d := range s.TopDecoys(10) {
		snap.TopDecoys = append(snap.TopDecoys, SnapshotDecoy{Target: d.Target, Hits: d.Hits})
	}
	return snap
}

// JSON renders the snapshot as indented JSON.
func (snap Snapshot) JSON() []byte {
	b, _ := json.MarshalIndent(snap, "", "  ")
	return append(b, '\n')
}

// Markdown renders the snapshot as a shareable report.
func (snap Snapshot) Markdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Heramba watch — snapshot\n\n")
	fmt.Fprintf(&b, "- **Generated:** %s\n", snap.GeneratedAt.Format(time.RFC3339))
	fmt.Fprintf(&b, "- **Mode:** %s\n", snap.Mode)
	fmt.Fprintf(&b, "- **Uptime:** %s\n", (time.Duration(snap.UptimeSecs) * time.Second).String())
	fmt.Fprintf(&b, "- **Threat:** %s (%.0f%%)\n", snap.ThreatLevel, snap.ThreatScore*100)
	fmt.Fprintf(&b, "- **Events:** %d across %d sources — %d swarm(s)\n\n", snap.Total, snap.Sources, snap.Swarms)

	if len(snap.ByKind) > 0 {
		fmt.Fprintf(&b, "## By kind\n\n| kind | count |\n|------|------:|\n")
		for _, k := range sortedKeysByCount(snap.ByKind) {
			fmt.Fprintf(&b, "| %s | %d |\n", k, snap.ByKind[k])
		}
		b.WriteString("\n")
	}
	if len(snap.TopSources) > 0 {
		fmt.Fprintf(&b, "## Top sources\n\n| source | severity | hits | decoys | canaries |\n|--------|----------|-----:|-------:|---------:|\n")
		for _, s := range snap.TopSources {
			fmt.Fprintf(&b, "| %s | %s | %d | %d | %d |\n", s.Addr, s.Severity, s.Hits, s.Decoys, s.Canaries)
		}
		b.WriteString("\n")
	}
	if len(snap.TopDecoys) > 0 {
		fmt.Fprintf(&b, "## Decoys touched\n\n| target | hits |\n|--------|-----:|\n")
		for _, d := range snap.TopDecoys {
			fmt.Fprintf(&b, "| %s | %d |\n", d.Target, d.Hits)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// sortedKeysByCount returns the map keys ordered by descending count, ties
// broken by key, for stable report output.
func sortedKeysByCount(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if m[keys[i]] != m[keys[j]] {
			return m[keys[i]] > m[keys[j]]
		}
		return keys[i] < keys[j]
	})
	return keys
}
