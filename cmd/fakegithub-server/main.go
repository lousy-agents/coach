// Command fakegithub-server runs internal/fakegithub as a standalone HTTP
// process, packaging the issue #79 Task 0.3 thin-offline-proof fixture
// (internal/acceptanceharness/thinproof) as an internal-network Compose
// service rather than an in-process httptest.Server.
//
// It listens on the address given by the -addr flag, or the ADDR
// environment variable if -addr is unset, defaulting to ":8080".
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/internal/acceptanceharness/thinproof"
	"github.com/lousy-agents/coach/internal/fakegithub"
)

func main() {
	addr := flag.String("addr", "", "address to listen on (overrides ADDR env var; default :8080)")
	flag.Parse()

	listenAddr := *addr
	if listenAddr == "" {
		listenAddr = os.Getenv("ADDR")
	}
	if listenAddr == "" {
		listenAddr = ":8080"
	}

	fixture := thinproof.BuildFixture()
	handler, recorder := fakegithub.Handler(&fixture)

	mux := http.NewServeMux()
	mux.Handle("/", handler)
	// /__test__/records is added by this command only, not by
	// internal/fakegithub itself -- it lets the thin Compose proof's runner
	// container (a separate OS process, so it cannot call recorder.Records()
	// in-process the way Task 1's fast test does) fetch the recorder's
	// contents over HTTP after driving requests through this server.
	// internal/fakegithub's own route table and contract (section 6,
	// docs/architecture/acceptance-harness.md) is unaffected: this route is
	// namespaced clearly out of the GitHub-shaped path space it serves.
	mux.HandleFunc("GET /__test__/records", recordsHandler(recorder))

	log.Printf("fakegithub-server: listening on %s", listenAddr)
	if err := http.ListenAndServe(listenAddr, mux); err != nil {
		log.Fatalf("fakegithub-server: %v", err)
	}
}

func recordsHandler(recorder *acceptanceharness.Recorder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(recorder.Records()); err != nil {
			http.Error(w, fmt.Sprintf("encoding records: %v", err), http.StatusInternalServerError)
		}
	}
}
