package acceptanceharness_test

import (
	"sync"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
)

type sigbodyfixtureAcceptanceTestisSafeUnderRaceAndEveryRecordIsP struct {
	iterationsPerWorker int
	recorder            *acceptanceharness.Recorder
	wg                  *sync.WaitGroup
}

func (sigRecv *sigbodyfixtureAcceptanceTestisSafeUnderRaceAndEveryRecordIsP) call(worker int) {
	defer sigRecv.wg.Done()
	for j := 0; j < sigRecv.iterationsPerWorker; j++ {
		sigRecv.recorder.
			Record(acceptanceharness.NewRequestRecord(
				"fixture-concurrent",
				"concurrent-scenario",
				"GET",
				"/concurrent",
				acceptanceharness.AuthModeNone,
			))
	}
}
