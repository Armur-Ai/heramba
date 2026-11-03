// Package coord is the R2 fair coordination-detection experiment. It generates
// matched, labeled cohorts (equalized request counts, time windows, and
// aggregate rate, so request volume cannot separate the classes), runs four
// detection methods and a battery of ablations over identical workloads, and
// reports recall / FPR / precision / PR-AUC with Wilson and bootstrap intervals,
// per-family confusion matrices, a session-granular detection latency, and
// throughput.
//
// The point of matching the workloads is scientific: if a request-rate-only
// heuristic cannot separate coordinated from independent traffic but the full
// detector can, the coordination features — not volume artifacts — are doing
// the work (hypothesis H1).
package coord

import (
	"fmt"
	"math/rand"
	"sort"
	"time"

	"github.com/Armur-Ai/heramba/internal/campaign"
	"github.com/Armur-Ai/heramba/internal/evidence"
	"github.com/Armur-Ai/heramba/internal/research"
)

// Params are the matched workload parameters shared by every cohort so that
// request volume and window are not confounders.
type Params struct {
	MatchedRequests int           // total requests per cohort (matched across families)
	Window          time.Duration // wall window every cohort spans (matched)
	SurfaceSize     int           // size of the wide enumeration surface
	KnownEndpoints  int           // size of the narrow known-endpoint set (benign/legit)
	BenignControls  int           // number of benign-control negative cohorts to add
	SyncWindow      time.Duration // coordinated starts fall within this
}

// DefaultParams returns calibrated, matched defaults.
func DefaultParams() Params {
	return Params{
		MatchedRequests: 200,
		Window:          5 * time.Minute,
		SurfaceSize:     40,
		KnownEndpoints:  6,
		BenignControls:  12,
		SyncWindow:      90 * time.Second,
	}
}

// Cohort is one labeled unit of classification: a set of sessions with a single
// ground-truth coordination label. The label is never shown to any detector.
type Cohort struct {
	Family      string
	ActorKind   string
	Seed        int64
	Coordinated bool // ground truth: a coordinated/deception-aware swarm
	Sessions    []campaign.Session
}

var origin = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// IsCoordinatedKind reports whether an actor kind is ground-truth coordinated.
func IsCoordinatedKind(kind string) bool {
	return kind == "coordinated" || kind == "deception_aware"
}

// GenerateCohort builds a deterministic labeled cohort for an actor family. The
// structural shape (synchrony, shared vs divided targeting, enumeration breadth,
// decoy contact) varies by class; the total request count and window are matched
// so aggregate rate is not discriminative.
func GenerateCohort(fam research.ScenarioFamily, actor research.ActorModel, seed int64, p Params) Cohort {
	rng := rand.New(rand.NewSource(seed))
	c := Cohort{
		Family:      fam.Name,
		ActorKind:   actor.Kind,
		Seed:        seed,
		Coordinated: IsCoordinatedKind(actor.Kind),
	}
	n := fam.ConcurrencyLevel
	if n < 1 {
		n = 1
	}
	surface := makeSurface("/scan/res-", p.SurfaceSize)
	known := makeSurface("/api/known-", p.KnownEndpoints)

	switch actor.Kind {
	case "coordinated", "deception_aware":
		// Synchronized starts; the surface is DIVIDED among sources (division of
		// labor) with a little shared overlap. Deception-aware swarms reject
		// obvious lures, so they make far fewer decoy hits — the hard case.
		counts := splitRequests(p.MatchedRequests, n)
		slice := len(surface) / n
		decoyAware := actor.Kind == "deception_aware"
		for i := 0; i < n; i++ {
			start := origin.Add(time.Duration(rng.Int63n(int64(p.SyncWindow))))
			lo := i * slice
			hi := lo + slice
			if i == n-1 {
				hi = len(surface)
			}
			tgts := targetsFromSlice(surface, lo, hi)
			// small shared overlap so shared-targeting also sometimes fires
			tgts[surface[(i+1)%len(surface)]]++
			deceptionHit := !decoyAware && i < 2
			if decoyAware && i == 0 {
				deceptionHit = false // explicitly rejects the lure
			}
			c.Sessions = append(c.Sessions, mkSession(fmt.Sprintf("swarm-%d-%d", seed, i), start, p.Window, counts[i], tgts, deceptionHit))
		}

	case "independent":
		// Several sources, DESYNCHRONIZED (spread across the whole window), each
		// enumerating the surface with the same wordlist (so target Jaccard is
		// high — this is what fools naive correlation) but with no shared timing.
		counts := splitRequests(p.MatchedRequests, n)
		for i := 0; i < n; i++ {
			start := origin.Add(time.Duration(rng.Int63n(int64(p.Window))))
			tgts := targetsFromSlice(surface, 0, min(len(surface), 12+rng.Intn(8)))
			c.Sessions = append(c.Sessions, mkSession(fmt.Sprintf("indep-%d-%d", seed, i), start, p.Window, counts[i], tgts, i == 0))
		}

	case "legit_automation":
		// High volume but NARROW, known endpoints; no decoy contact; no synchrony.
		counts := splitRequests(p.MatchedRequests, n)
		for i := 0; i < n; i++ {
			start := origin.Add(time.Duration(rng.Int63n(int64(p.Window))))
			tgts := targetsFromSlice(known, 0, len(known))
			c.Sessions = append(c.Sessions, mkSession(fmt.Sprintf("legit-%d-%d", seed, i), start, p.Window, counts[i], tgts, false))
		}

	case "human", "scripted":
		// Single source, exploratory/sequential; narrow-to-moderate surface.
		start := origin.Add(time.Duration(rng.Int63n(int64(p.Window))))
		width := p.KnownEndpoints
		if actor.Kind == "scripted" {
			width = min(len(surface), 16)
		}
		tgts := targetsFromSlice(surface, 0, width)
		c.Sessions = append(c.Sessions, mkSession(fmt.Sprintf("solo-%d", seed), start, p.Window, p.MatchedRequests, tgts, actor.Kind == "scripted"))
	}

	sort.SliceStable(c.Sessions, func(i, j int) bool { return c.Sessions[i].Start.Before(c.Sessions[j].Start) })
	return c
}

// GenerateBenignCohort produces a realistic benign-client cohort as a negative
// control: a handful of independent legitimate clients hitting a few known
// endpoints, spread across the window, never touching a decoy. These are the
// rate-matched benign controls the FPR is measured against. They are deliberately
// a *separate* cohort rather than mixed into an attacker cohort, because the
// swarm detector classifies a candidate cluster — in production the correlator
// has already separated benign traffic into its own cluster.
func GenerateBenignCohort(seed int64, p Params) Cohort {
	rng := rand.New(rand.NewSource(seed))
	known := makeSurface("/api/known-", p.KnownEndpoints)
	c := Cohort{Family: "benign-control", ActorKind: "benign", Seed: seed, Coordinated: false}
	nClients := 2 + rng.Intn(3)
	counts := splitRequests(p.MatchedRequests, nClients)
	for i := 0; i < nClients; i++ {
		start := origin.Add(time.Duration(rng.Int63n(int64(p.Window))))
		tgts := targetsFromSlice(known, 0, 2+rng.Intn(min(3, len(known))))
		c.Sessions = append(c.Sessions, mkSession(fmt.Sprintf("benign-%d-%d", seed, i), start, p.Window, counts[i], tgts, false))
	}
	sort.SliceStable(c.Sessions, func(i, j int) bool { return c.Sessions[i].Start.Before(c.Sessions[j].Start) })
	return c
}

func mkSession(src string, start time.Time, window time.Duration, requests int, targets map[string]int, decoy bool) campaign.Session {
	if requests < 1 {
		requests = 1
	}
	ids := make([]evidence.ID, requests)
	for i := range ids {
		ids[i] = evidence.ID(fmt.Sprintf("ev_%s_%d", src, i))
	}
	return campaign.Session{
		ID:           evidence.ID("sess_" + src),
		SourceKey:    src,
		Start:        start,
		End:          start.Add(window),
		EventIDs:     ids,
		Targets:      targets,
		DeceptionHit: decoy,
	}
}

func makeSurface(prefix string, n int) []string {
	out := make([]string, n)
	for i := 0; i < n; i++ {
		out[i] = fmt.Sprintf("%s%d", prefix, i)
	}
	return out
}

func targetsFromSlice(surface []string, lo, hi int) map[string]int {
	m := map[string]int{}
	for i := lo; i < hi && i < len(surface); i++ {
		m[surface[i]] = 1
	}
	return m
}

// splitRequests partitions total requests across n sources so the sum is
// exactly total (the remainder is spread over the first sources), keeping the
// per-cohort request count matched regardless of concurrency.
func splitRequests(total, n int) []int {
	if n < 1 {
		n = 1
	}
	out := make([]int, n)
	base := total / n
	rem := total % n
	for i := range out {
		out[i] = base
		if i < rem {
			out[i]++
		}
	}
	return out
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
