package agentloopharness_test

import (
	"context"
	"encoding/json"
	"sync"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/acceptanceharness/agentloopharness"
)

func body_registryAcceptanceTest_isSafeUnderRaceAndEveryCallIsRecorded_113() {
	registry := &agentloopharness.RecordingToolRegistry{}
	registry.Register("concurrent_tool", func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
		return json.RawMessage(`{}`), nil
	})

	const workers = 8
	const iterationsPerWorker = 50
	const total = workers * iterationsPerWorker

	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go (&sigbodyregistryAcceptanceTestisSafeUnderRaceAndEveryCallIsRe{iterationsPerWorker: iterationsPerWorker, registry: registry, wg: &wg}).call(i)
	}
	wg.Wait()

	Expect(registry.Calls()).To(HaveLen(total))
}
