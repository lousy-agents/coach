package acceptanceharness_test

import (
	"sync"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
)

func body_fixtureAcceptanceTest_isSafeUnderRaceAndEveryRecordIsPreservedMirrorsC_116() {
	var recorder acceptanceharness.Recorder

	const workers = 8
	const iterationsPerWorker = 50
	const total = workers * iterationsPerWorker

	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go (&sigbodyfixtureAcceptanceTestisSafeUnderRaceAndEveryRecordIsP{iterationsPerWorker: iterationsPerWorker, recorder: recorder, wg: wg}).call(i)
	}
	wg.Wait()

	Expect(recorder.Records()).To(HaveLen(total))
}
