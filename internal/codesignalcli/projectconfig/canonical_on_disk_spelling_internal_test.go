package projectconfig

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

func probeOver(disk foldingDisk) func(string, string) error {
	return func(_, repoPath string) error { return checkRegularFileThrough(disk, repoPath) }
}

// Discovery asks the disk only once the revision commits nothing at the default policy's path in any spelling.
// A disk that folds case then shows the path in whatever spelling it keeps, and that is not a refusal: there is
// no policy for a spelling to be wrong about, and the scan goes on without one, as v0.11.0 did. A directory
// another tool keeps beside the policy's, or an untracked file in another case, must not stop every run.
func TestADirectoryTheDiskSpellsInAnotherCaseWithNoPolicyInItIsNothing(t *testing.T) {
	for name, held := range map[string][]string{
		"an untracked directory with another tool's file": {".Lousy-Agents/doctor.json"},
		"an untracked directory with another file":        {".Lousy-Agents/other"},
		"the right directory with another file in it":     {".lousy-agents/doctor.json"},
	} {
		t.Run(name, func(t *testing.T) { requireNothingAtTheDefaultPath(t, held...) })
	}
}

func requireNothingAtTheDefaultPath(t *testing.T, held ...string) {
	t.Helper()

	selection, err := canonicalOnDiskThrough(probeOver(foldingDiskHolding(held...)), "root")

	if err != nil || selection.UncommittedCanonical || selection.UnusableCanonical != nil || selection.Fatal != nil {
		t.Errorf("canonicalOnDiskThrough = %+v, %v; want a scan that goes on, with no policy and nothing to say", selection, err)
	}
}

func TestAFileTheDiskSpellsInAnotherCaseIsNamedAndNotRead(t *testing.T) {
	for held, spelling := range map[string]string{
		".lousy-agents/Coach.json": ".lousy-agents/Coach.json",
		".Lousy-Agents/coach.json": ".Lousy-Agents",
		".LOUSY-AGENTS/COACH.JSON": ".LOUSY-AGENTS",
		".Lousy-Agents/Coach.json": ".Lousy-Agents",
	} {
		t.Run(held, func(t *testing.T) { requireTheSpellingNamed(t, held, spelling) })
	}
}

func requireTheSpellingNamed(t *testing.T, held, spelling string) {
	t.Helper()

	selection, err := canonicalOnDiskThrough(probeOver(foldingDiskHolding(held)), "root")

	if err != nil || selection.Fatal != nil || selection.UncommittedCanonical || causeOf(selection.UnusableCanonical) != projectreadiness.CauseNameCaseDiffers {
		t.Fatalf("canonicalOnDiskThrough = %+v, %v; want a scan that goes on, with a path that cannot be the policy for its spelling", selection, err)
	}
	message := UnusableCanonicalMessage(selection.UnusableCanonical)
	if !strings.Contains(message, "cause=name_case_differs") || !strings.Contains(message, `"`+spelling+`"`) || strings.Contains(message, "--project-config") {
		t.Errorf("message %q; want the cause, the spelling %q, and no flag the user did not pass", message, spelling)
	}
}

// What is at the default path in the spelling Coach reads is still a policy that is not committed.
func TestAFileSpelledAsCoachReadsItIsStillUncommitted(t *testing.T) {
	selection, err := canonicalOnDiskThrough(probeOver(foldingDiskHolding(".lousy-agents/coach.json")), "root")

	if err != nil || !selection.UncommittedCanonical || selection.UnusableCanonical != nil {
		t.Errorf("canonicalOnDiskThrough = %+v, %v; want an uncommitted policy", selection, err)
	}
}
