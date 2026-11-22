package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Armur-Ai/heramba/internal/alert"
	"github.com/Armur-Ai/heramba/internal/event"
	"github.com/Armur-Ai/heramba/internal/poison"
	"github.com/Armur-Ai/heramba/internal/tarpit"
	"github.com/Armur-Ai/heramba/internal/tripwire"
)

// runTrap powers `heramba trap` — a standalone active-defense endpoint that
// chains the three new planes together: it plants signed trip-URLs in bait
// surfaces (robots.txt, sitemap, a JS config blob, fake API docs) and in the
// comments/headers of ordinary responses; the instant a client follows one —
// which only a crawler or a snooping attacker ever does — that source is
// confirmed bad, and every further request from it is sunk into an escalating
// tarpit, an endless labyrinth of fake pages, and watermarked poison data.
//
// Everything is defensive: we only ever serve content from our own endpoint or
// slow down the attacker's own requests to us. Nothing is sent at the client.
func runTrap(args []string) {
	fs := flag.NewFlagSet("trap", flag.ExitOnError)
	listen := fs.String("listen", "127.0.0.1:8080", "listen address")
	seed := fs.Int64("seed", 1, "seed for deterministic bait tokens, labyrinth, and poison")
	base := fs.String("base", "", "public base URL for bait links (default: http://<listen>)")
	format := fs.String("alert", "text", "alert format: text|jsonl")
	baseDelay := fs.Duration("delay", 250*time.Millisecond, "first-offense tarpit drip interval")
	maxDelay := fs.Duration("max-delay", 3*time.Second, "cap on the tarpit drip interval")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: heramba trap [flags]\n\n"+
			"A standalone active-defense endpoint: trip-wire bait in front, tarpit +\n"+
			"labyrinth + watermarked poison for anything that trips it.\n\nflags:\n")
		fs.PrintDefaults()
	}
	_ = fs.Parse(args)

	if *base == "" {
		*base = "http://" + *listen
	}
	var sink alert.Sink
	if *format == "jsonl" {
		sink = alert.NewJSONL(os.Stdout)
	} else {
		sink = alert.NewText(os.Stdout)
	}

	th, err := newTrapHandler(trapConfig{
		Seed: *seed, Base: *base, Sink: sink,
		BaseDelay: *baseDelay, MaxDelay: *maxDelay,
		Jitter: 150 * time.Millisecond, ChunkSize: 1,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "heramba trap:", err)
		os.Exit(1)
	}

	srv := &http.Server{Addr: *listen, Handler: th, ReadHeaderTimeout: 10 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Printf("heramba trap — active defense on http://%s (base=%s)\n", *listen, *base)
	fmt.Println("  bait surfaces:", strings.Join(th.baitPaths(), " "))
	fmt.Println("  trip one and every later request is tarpitted + poisoned. Ctrl-C to stop.")
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintln(os.Stderr, "heramba trap:", err)
			stop()
		}
	}()
	<-ctx.Done()
	shutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutCtx)
}

// trapConfig configures a trap handler.
type trapConfig struct {
	Seed      int64
	Base      string
	Sink      alert.Sink
	BaseDelay time.Duration
	MaxDelay  time.Duration
	Jitter    time.Duration
	ChunkSize int
}

// trap is the composed active-defense handler.
type trap struct {
	planter   *tripwire.Planter
	matcher   *tripwire.Matcher
	esc       *tarpit.Escalator
	lab       *tarpit.Labyrinth
	gen       *poison.Generator
	sink      alert.Sink
	chunk     int
	jitter    time.Duration
	bait      map[string]tripwire.Bait // conventional path -> served bait body
	linkHdr   string                   // Link: header planted on benign responses
	htmlBait  string                   // HTML comment bait inserted into benign pages
	confirmed sync.Map                 // source -> struct{}: sources that tripped a wire
}

// newTrapHandler builds the composed handler. The signer key, labyrinth, and
// poison generator are all derived from the seed, so bait tokens stay valid and
// reproducible across restarts with the same seed.
func newTrapHandler(cfg trapConfig) (*trap, error) {
	if cfg.Sink == nil {
		cfg.Sink = alert.NewText(os.Stdout)
	}
	if cfg.ChunkSize <= 0 {
		cfg.ChunkSize = 1
	}
	key := sha256.Sum256([]byte(fmt.Sprintf("heramba-trap:%d", cfg.Seed)))
	signer, err := tripwire.NewSigner(key[:])
	if err != nil {
		return nil, err
	}
	planter := tripwire.NewPlanter(signer, cfg.Base)
	marker, err := poison.NewMarker(key[:])
	if err != nil {
		return nil, err
	}
	t := &trap{
		planter:  planter,
		matcher:  tripwire.NewMatcher(signer),
		esc:      tarpit.NewEscalator(cfg.BaseDelay, cfg.MaxDelay, tarpit.Exponential),
		lab:      tarpit.NewLabyrinth(cfg.Seed, "/internal", 6, 8),
		gen:      poison.NewGenerator(cfg.Seed, marker),
		sink:     cfg.Sink,
		chunk:    cfg.ChunkSize,
		jitter:   cfg.Jitter,
		bait:     map[string]tripwire.Bait{},
		linkHdr:  planter.LinkHeader().Body,
		htmlBait: planter.HTMLComments().Body,
	}
	t.bait["/robots.txt"] = planter.RobotsTxt()
	t.bait["/sitemap.xml"] = planter.SitemapXML()
	t.bait["/assets/app.config.js"] = planter.JSConfig()
	t.bait["/api/docs"] = planter.APIDocs()
	return t, nil
}

// baitPaths lists the conventional paths that serve bait, for the banner.
func (t *trap) baitPaths() []string {
	out := make([]string, 0, len(t.bait))
	for p := range t.bait {
		out = append(out, p)
	}
	// deterministic order for a stable banner / test
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}

func (t *trap) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	src := trapClientIP(r)

	// Already confirmed bad: sink everything into the tarpit.
	if _, bad := t.confirmed.Load(src); bad {
		t.serveTrapped(w, r, src)
		return
	}
	// Did this request follow a trip-URL? (cryptographically verified.)
	if ev, ok := t.matcher.InspectRequest(r, src); ok {
		_ = t.sink.Emit(ev)
		t.confirmed.Store(src, struct{}{})
		t.serveTrapped(w, r, src)
		return
	}
	// Bait surfaces are served verbatim (they carry the trip-URLs).
	if b, ok := t.bait[r.URL.Path]; ok {
		if b.ContentType != "" {
			w.Header().Set("Content-Type", b.ContentType)
		}
		_, _ = w.Write([]byte(b.Body))
		return
	}
	// Everything else: a plausible page seeded with comment + header bait.
	t.serveBenign(w, r)
}

// serveTrapped wastes a confirmed-bad source's time: it escalates the per-source
// delay, then drips either watermarked poison (for secret-looking paths) or the
// endless labyrinth, honoring the request context so nothing leaks.
func (t *trap) serveTrapped(w http.ResponseWriter, r *http.Request, src string) {
	delay := t.esc.Offense(src)
	tp := tarpit.New(tarpit.Config{
		ChunkSize: t.chunk,
		Interval:  delay,
		Jitter:    t.jitter,
		MaxTotal:  30 * time.Second, // bound any single response
	})
	if looksLikeSecretPath(r.URL.Path) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		body := []byte(t.gen.ConfigEnv()) // inert, watermarked fake secrets
		_, _ = tp.Drip(r.Context(), w, body)
		_ = t.sink.Emit(event.New(event.KindCounterDeployed, event.SeverityMedium, src, r.URL.Path,
			"served watermarked poison config to confirmed-bad source").
			WithLabel("plane", "poison"))
		return
	}
	t.lab.Handler(tp, nil).ServeHTTP(w, r)
}

// serveBenign returns an innocuous page that nonetheless plants an HTML-comment
// trip-link and a Link-header trip-URL, so an enumerating crawler finds bait in
// ordinary responses too.
func (t *trap) serveBenign(w http.ResponseWriter, _ *http.Request) {
	if t.linkHdr != "" {
		w.Header().Set("Link", t.linkHdr)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, "<!doctype html><html><head><title>Welcome</title></head><body>\n"+
		"<h1>It works</h1>\n<p>Nothing to see here.</p>\n%s\n</body></html>\n", t.htmlBait)
}

// looksLikeSecretPath is true for paths that an attacker probes hoping to find
// credentials — exactly where serving poison is most useful.
func looksLikeSecretPath(path string) bool {
	p := strings.ToLower(path)
	for _, marker := range []string{".env", "secret", "credential", "config", ".git", "backup", "dump", ".aws", "id_rsa"} {
		if strings.Contains(p, marker) {
			return true
		}
	}
	return false
}

// trapClientIP extracts the client IP from the request (host part of
// RemoteAddr), falling back to the raw value.
func trapClientIP(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
