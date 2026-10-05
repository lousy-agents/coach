package codesignalcli

import (
	"io"

	"testing"
	"time"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func body_projectConfigAuthoringPart13Test_20(t *testing.T, in io.Reader, discovered projectmodel.TSRootDiscoveryResult, watchdog time.Duration) {
	t.Helper()
	type outcome struct {
		result AuthoringResult
	}
	done := make(chan outcome, 1)
	go func() {
		done <- outcome{AuthorProjectConfig("", in, io.Discard, io.Discard, discovered, "", false)}
	}()

	select {
	case o := <-done:
		if o.result.Approved {
			t.Fatalf("expected an exhausted/erroring read at the approval gate to not approve, got Approved = true")
		}
	case <-time.After(watchdog):
		t.Fatalf("AuthorProjectConfig did not return within %s: the approval gate is hanging on exhausted/erroring input", watchdog)
	}
}
