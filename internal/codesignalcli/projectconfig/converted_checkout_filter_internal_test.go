package projectconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

// A filter attribute selects a program, and no program runs on a scan: the file is taken to differ and git is never
// asked to hash it, so the filter does not run.
func TestAFileAFilterAttributeSelectsAProgramForIsNeverConverted(t *testing.T) {
	repo, objectID := aConvertedCheckout(t, "core.autocrlf", "true")
	mark := filepath.Join(t.TempDir(), "ran")
	script := filepath.Join(t.TempDir(), "clean.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho ran >> '"+mark+"'\ncat\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	gitfixture.Git(t, repo, "config", "filter.probe.clean", script)
	writeAttributes(t, repo, "cfg/p.json filter=probe\n")

	requireConverted(t, repo, objectID, false)
	if _, err := os.Stat(mark); err == nil {
		t.Errorf("the clean filter ran")
	}
}

// A filter that is unset names no program.
func TestAFilterThatIsUnsetSelectsNoProgram(t *testing.T) {
	repo, objectID := aConvertedCheckout(t, "core.autocrlf", "true")
	writeAttributes(t, repo, "cfg/p.json -filter\n")

	requireConverted(t, repo, objectID, true)
}
