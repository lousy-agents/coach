package main

import (
	"fmt"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/coachapi/worker"
	"github.com/lousy-agents/coach/pkg/githubingest"
)

func body_handlerAcceptanceTest_keepsErrAuthAndErrTooLargePermanent_105() {
	for _, sent := range []error{githubingest.ErrAuth, githubingest.ErrTooLarge} {
		err := classifyBaselineHandlerError(fmt.Errorf("coachapi: baseline fetch failed: %w", sent))
		Expect(worker.IsRetryable(err)).To(BeFalse(), "sentinel %v", sent)
	}
}
