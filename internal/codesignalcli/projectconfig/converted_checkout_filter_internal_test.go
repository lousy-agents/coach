package projectconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

// aFilterProbe configures the program of a filter driver, which leaves a mark where it runs and passes its input through.
// It returns the path of the mark.
func aFilterProbe(t *testing.T, repo, driver, program string) (mark string) {
	t.Helper()
	mark = filepath.Join(t.TempDir(), "ran")
	script := filepath.Join(t.TempDir(), "program.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho ran >> '"+mark+"'\ncat\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	gitfixture.Git(t, repo, "config", "filter."+driver+"."+program, script)
	return mark
}

func requireNoProgramRan(t *testing.T, mark string) {
	t.Helper()
	if _, err := os.Stat(mark); err == nil {
		t.Errorf("the filter's program ran")
	}
}

// A filter attribute selects a program, and no program runs on a scan: the file is taken to differ and git is never
// asked to hash it, so the filter does not run.
func TestAFileAFilterAttributeSelectsAProgramForIsNeverConverted(t *testing.T) {
	repo, objectID := aConvertedCheckout(t, "core.autocrlf", "true")
	mark := aFilterProbe(t, repo, "probe", "clean")
	writeAttributes(t, repo, "cfg/p.json filter=probe\n")

	requireConverted(t, repo, objectID, false)
	requireNoProgramRan(t, mark)
}

// git check-attr prints a string value as it is written, so a driver that is named unset or unspecified prints what an
// attribute that is unset, or not set at all, prints. It is still a filter, and its program must not run.
func TestADriverNamedLikeNoFilterIsStillAFilterAndItsProgramNeverRuns(t *testing.T) {
	for _, c := range []struct{ driver, program string }{
		{"unset", "clean"}, {"unset", "process"}, {"unspecified", "clean"}, {"unspecified", "process"},
	} {
		t.Run(c.driver+" "+c.program, func(t *testing.T) { assertTheNamedDriverNeverRuns(t, c.driver, c.program) })
	}
}

func assertTheNamedDriverNeverRuns(t *testing.T, driver, program string) {
	t.Helper()
	repo, objectID := aConvertedCheckout(t, "core.autocrlf", "true")
	mark := aFilterProbe(t, repo, driver, program)
	writeAttributes(t, repo, "cfg/p.json filter="+driver+"\n")

	requireConverted(t, repo, objectID, false)
	requireNoProgramRan(t, mark)
}

// A filter attribute that is unset prints as the driver named unset does, and the two cannot be told apart, so it is taken
// to select a program too: the file is called changed, which fails safe.
func TestAFilterAttributeThatIsUnsetIsTakenAsSelectedBecauseGitPrintsItLikeADriverNamedUnset(t *testing.T) {
	repo, objectID := aConvertedCheckout(t, "core.autocrlf", "true")
	writeAttributes(t, repo, "cfg/p.json -filter\n")

	requireConverted(t, repo, objectID, false)
}

// A macro that sets filter selects the program as a plain attribute does.
func TestAFilterSelectedThroughAMacroNeverRuns(t *testing.T) {
	repo, objectID := aConvertedCheckout(t, "core.autocrlf", "true")
	mark := aFilterProbe(t, repo, "probe", "clean")
	writeAttributes(t, repo, "[attr]viaMacro filter=probe\ncfg/p.json viaMacro\n")

	requireConverted(t, repo, objectID, false)
	requireNoProgramRan(t, mark)
}

// A driver that is configured and that no attribute selects is no filter: the checkout is converted, and the program is
// not run for it.
func TestADriverThatNoAttributeSelectsIsNoFilter(t *testing.T) {
	repo, objectID := aConvertedCheckout(t, "core.autocrlf", "true")
	mark := aFilterProbe(t, repo, "unset", "clean")

	requireConverted(t, repo, objectID, true)
	requireNoProgramRan(t, mark)
}

// Only an attribute that is named filter is a filter: a value that is the word does not make one, so the checkout is still
// converted.
func TestAnAttributeWhoseValueIsTheWordFilterIsNoFilter(t *testing.T) {
	repo, objectID := aConvertedCheckout(t, "core.autocrlf", "true")
	writeAttributes(t, repo, "cfg/p.json diff=filter\n")

	requireConverted(t, repo, objectID, true)
}
