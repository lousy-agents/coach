package agentloopharness_test

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/lousy-agents/coach/internal/acceptanceharness/agentloopharness"
)

type sigbodyregistryAcceptanceTestisSafeUnderRaceAndEveryCallIsRe struct {
	iterationsPerWorker int
	registry            *agentloopharness.
				RecordingToolRegistry
	wg sync.
		WaitGroup
}

func (sigRecv *sigbodyregistryAcceptanceTestisSafeUnderRaceAndEveryCallIsRe) call(worker int) {
	defer sigRecv.wg.Done()
	for j := 0; j < sigRecv.iterationsPerWorker; j++ {
		_, _ = sigRecv.registry.Call(context.Background(), agentloopharness.CallSourceModel, "concurrent_tool", json.RawMessage(`{}`))
	}
}
