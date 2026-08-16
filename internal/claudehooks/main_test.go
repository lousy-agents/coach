package claudehooks

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMain pins COACH_HOOK_TRACE for every test in this package, Ginkgo and
// stdlib alike, so no hook subprocess can fall through to the marker-file path
// and append to the repository's own .coach-hook-trace.tsv.
//
// That file is what AGENTS.md tells a human to read to find out what a run
// actually did, and rows written by a test are indistinguishable from rows
// written by a session. A per-test opt-in is the wrong shape for this: the
// tests that pollute are the ones that never thought about tracing at all, so
// the default has to be safe rather than the exception. Individual specs still
// override it -- with their own path, or with an empty value to exercise the
// not-requested path.
func TestMain(m *testing.M) {
	if _, pinned := os.LookupEnv("COACH_HOOK_TRACE"); !pinned {
		dir, err := os.MkdirTemp("", "coach-hook-trace")
		if err != nil {
			panic("could not create a trace sandbox: " + err.Error())
		}
		defer os.RemoveAll(dir)
		if err := os.Setenv("COACH_HOOK_TRACE", filepath.Join(dir, "trace.tsv")); err != nil {
			panic("could not pin COACH_HOOK_TRACE: " + err.Error())
		}
	}
	m.Run()
}
