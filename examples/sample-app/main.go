// Command sample-app is a trivial upstream for trying Heramba locally. It has a
// couple of real endpoints and nothing secret. Run it, then run heramba in
// front of it (see examples/.heramba.yml).
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = "127.0.0.1:3000"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "sample-app: hello from %s\n", r.URL.Path)
	})
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	log.Printf("sample-app listening on %s", addr)
	srv := &http.Server{Addr: addr, Handler: mux}
	log.Fatal(srv.ListenAndServe())
}
