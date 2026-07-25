// Command model-stub serves the compose OpenAI-compatible HTTP stub from
// internal/modelgateway so core smoke can exercise worker → Envoy AI Gateway →
// backend without model weights. Wire path ownership stays in modelgateway.
package main

import (
	"errors"
	"log"
	"net/http"
	"os"

	"github.com/lousy-agents/coach/internal/modelgateway"
)

const defaultAddr = ":8090"

func main() {
	addr := os.Getenv("COACH_MODEL_STUB_ADDR")
	if addr == "" {
		addr = defaultAddr
	}
	log.Printf("model-stub: listening on %s", addr)
	if err := modelgateway.ListenAndServeHTTPStub(addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("model-stub: %v", err)
	}
}
