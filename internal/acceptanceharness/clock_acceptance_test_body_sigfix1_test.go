package acceptanceharness_test

import (
	"sync"
	"time"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
)

type sigbodyclockAcceptanceTestfiresEveryRegisteredWaiterExactlyO struct {
	iterationsPerWorker int
	clock               *acceptanceharness.
				FakeClock
	fired chan time.
		Time
	producers sync.
			WaitGroup
}

func (sigRecv *sigbodyclockAcceptanceTestfiresEveryRegisteredWaiterExactlyO) call() {
	defer sigRecv.producers.Done()
	for j := 0; j < sigRecv.iterationsPerWorker; j++ {
		ch := sigRecv.clock.After(time.Millisecond)

		_ = sigRecv.clock.Now()
		sigRecv.fired <- <-ch
	}
}
