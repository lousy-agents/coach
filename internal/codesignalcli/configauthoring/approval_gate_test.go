package configauthoring

import (
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func TestAuthorProjectConfig_ApprovalGateRequiresExactApprovalToken(t *testing.T) {
	discovered := projectmodel.TSRootDiscoveryResult{Roots: []string{"apps/api"}, Complete: true}

	declineCases := []struct {
		name   string
		answer string
	}{
		{name: "declines with a blank answer", answer: ""},
		{name: "declines with an unrelated word", answer: "no"},
		{name: "declines with a near-miss token", answer: "approved"},
	}

	for _, tc := range declineCases {
		t.Run(tc.name, func(t *testing.T) {
			checkApprovalGateRequiresExactApprovalToken(t, discovered, tc)
		})
	}

	t.Run("approves with the exact approval token", func(t *testing.T) {
		approvesExactApprovalToken(t, discovered)
	})

	t.Run("approves case-insensitively", func(t *testing.T) {
		approvesCaseInsensitively(t, discovered)
	})
}

func checkApprovalGateRequiresExactApprovalToken(t *testing.T, discovered projectmodel.TSRootDiscoveryResult, tc struct {
	name   string
	answer string
}) {
	result, _ := runAuthoring(discovered,
		"1",
		"api", "apps/api",
		"",
		"",
		"",
		tc.answer,
	)
	if result.Approved {
		t.Fatalf("expected Approved = false for answer %q, got true", tc.answer)
	}
	t.Run("declining is not a cancellation", func(t *testing.T) {
		decliningNotCancellation(t, tc, result)
	})
}

func decliningNotCancellation(t *testing.T, tc struct {
	name   string
	answer string
}, result Result) {
	if result.Cancelled {
		t.Fatalf("expected Cancelled = false for declined answer %q (a later write stage must gate on Approved, not !Cancelled), got true", tc.answer)
	}
}

func approvesExactApprovalToken(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	result, _ := runAuthoring(discovered,
		"1",
		"api", "apps/api",
		"",
		"",
		"",
		"approve",
	)
	if !result.Approved {
		t.Fatalf("expected Approved = true for the exact approval token, got false")
	}
}

func approvesCaseInsensitively(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	result, _ := runAuthoring(discovered,
		"1",
		"api", "apps/api",
		"",
		"",
		"",
		"APPROVE",
	)
	if !result.Approved {
		t.Fatalf("expected Approved = true for a case-insensitive approval token, got false")
	}
}

func TestAuthorProjectConfig_ApprovalGateTerminatesPromptlyOnExhaustedOrErroringInput(t *testing.T) {
	discovered := projectmodel.TSRootDiscoveryResult{Complete: true}
	const watchdog = 3 * time.Second

	assertNotApprovedBeforeWatchdog := func(t *testing.T, in io.Reader) {
		checkApprovalGateTerminatesPromptlyExhaustedErroring(t, in, discovered, watchdog)
	}

	t.Run("persistent non-EOF read error does not approve and returns before watchdog", func(t *testing.T) {
		in := &persistentErrorReader{data: []byte("root\napiLayer\napps/api\n\n\n\n"), err: errors.New("simulated non-EOF read error (e.g. EBADF/EIO)")}
		assertNotApprovedBeforeWatchdog(t, in)
	})

	t.Run("clean EOF at the approval prompt does not approve and returns before watchdog", func(t *testing.T) {
		in := strings.NewReader("root\napiLayer\napps/api\n\n\n\n")
		assertNotApprovedBeforeWatchdog(t, in)
	})
}

func checkApprovalGateTerminatesPromptlyExhaustedErroring(t *testing.T, in io.Reader, discovered projectmodel.TSRootDiscoveryResult, watchdog time.Duration) {
	t.Helper()
	type outcome struct {
		result Result
	}
	done := make(chan outcome, 1)
	go func() {
		done <- outcome{Author("", in, io.Discard, io.Discard, discovered, "", false)}
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
