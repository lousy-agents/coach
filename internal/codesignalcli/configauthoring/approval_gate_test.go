package configauthoring

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
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
			body_projectConfigAuthoringPart10Test_24(t, discovered, tc)
		})
	}

	t.Run("approves with the exact approval token", func(t *testing.T) {
		body_projectConfigAuthoringPart10Test_approvesWithTheExactApprovalToken_44(t, discovered)
	})

	t.Run("approves case-insensitively", func(t *testing.T) {
		body_projectConfigAuthoringPart10Test_approvesCaseInsensitively_58(t, discovered)
	})
}

func body_projectConfigAuthoringPart10Test_24(t *testing.T, discovered projectmodel.TSRootDiscoveryResult, tc struct {
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
		body_projectConfigAuthoringPart10Test_decliningIsNotACancellation_36(t, tc, result)
	})
}

func body_projectConfigAuthoringPart10Test_decliningIsNotACancellation_36(t *testing.T, tc struct {
	name   string
	answer string
}, result Result) {
	if result.Cancelled {
		t.Fatalf("expected Cancelled = false for declined answer %q (a later write stage must gate on Approved, not !Cancelled), got true", tc.answer)
	}
}

func body_projectConfigAuthoringPart10Test_approvesWithTheExactApprovalToken_44(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
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

func body_projectConfigAuthoringPart10Test_approvesCaseInsensitively_58(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
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
		body_projectConfigAuthoringPart13Test_20(t, in, discovered, watchdog)
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

func body_projectConfigAuthoringPart13Test_20(t *testing.T, in io.Reader, discovered projectmodel.TSRootDiscoveryResult, watchdog time.Duration) {
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

func TestAuthorProjectConfig_DeclinedApprovalNeverReachesTheWritePath(t *testing.T) {
	discovered := projectmodel.TSRootDiscoveryResult{Roots: []string{"apps/api"}, Complete: true}

	t.Run("with --output set, no file is created", func(t *testing.T) {
		body_projectConfigAuthoringPart9Test_withOutputSetNoFileIsCreated_18(t, discovered)
	})

	t.Run("with --output unset, nothing beyond the interactive text is written to out", func(t *testing.T) {
		body_projectConfigAuthoringPart9Test_withOutputUnsetNothingBeyondTheInteractiveTextIs_50(t, discovered)
	})
}

func body_projectConfigAuthoringPart9Test_withOutputSetNoFileIsCreated_18(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	dir := t.TempDir()
	outputPath := "project-config.json"
	out := &bytes.Buffer{}
	in := strings.NewReader(strings.Join([]string{
		"1",
		"domain", "internal/domain",
		"",
		"",
		"",
		"nope",
	}, "\n") + "\n")

	result := Author(dir, in, out, out, discovered, outputPath, true)

	if result.Approved {
		t.Fatalf("expected Approved = false, got true")
	}
	if result.Document != nil {
		t.Fatalf("expected no Document for a declined approval, got %s", result.Document)
	}
	if result.OutputExists {
		t.Fatalf("expected OutputExists = false: the write path must never run for a declined approval")
	}
	if result.WriteError != nil {
		t.Fatalf("expected WriteError = nil: the write path must never run for a declined approval, got %v", result.WriteError)
	}
	if _, err := os.Stat(filepath.Join(dir, outputPath)); !os.IsNotExist(err) {
		t.Fatalf("expected no file to be created for a declined approval, stat err = %v", err)
	}
}

func body_projectConfigAuthoringPart9Test_withOutputUnsetNothingBeyondTheInteractiveTextIs_50(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	out := &bytes.Buffer{}
	in := strings.NewReader(strings.Join([]string{
		"1",
		"domain", "internal/domain",
		"",
		"",
		"",
		"nope",
	}, "\n") + "\n")

	result := Author(t.TempDir(), in, out, out, discovered, "", false)

	if result.Approved {
		t.Fatalf("expected Approved = false, got true")
	}
	if tail := tailAfterLastPrompt(out.String()); tail != "" {
		t.Fatalf("expected nothing written after the approval prompt for a declined approval, got %q", tail)
	}
}
