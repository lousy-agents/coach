//go:build unix

package sourcescope

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// goThatRecordsHowItWasRun puts a go on PATH that writes its arguments and the toolchain setting it saw to the
// file it returns, and prints no packages.
func goThatRecordsHowItWasRun(t *testing.T) string {
	t.Helper()
	bin := t.TempDir()
	record := filepath.Join(t.TempDir(), "record")
	script := "#!/bin/sh\nprintf 'args=%s\\ntoolchain=%s\\n' \"$*\" \"$GOTOOLCHAIN\" > '" + record + "'\n"
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return record
}

// go list runs in a directory of committed bytes. It must not stamp version control information, which runs
// git in the directory and so the program a committed .git/config names, and it must not fetch and run a
// toolchain that a committed go.mod names. The caller's own settings do not change either.
func TestGoListInTheSnapshotStampsNoVersionControlAndFetchesNoToolchain(t *testing.T) {
	record := goThatRecordsHowItWasRun(t)
	t.Setenv("GOTOOLCHAIN", "auto")
	t.Setenv("GOFLAGS", "-buildvcs=true")
	snapshot := t.TempDir()

	if _, err := goProductionFiles(snapshot, snapshot, snapshot, "./..."); err != nil {
		t.Fatalf("goProductionFiles: %v", err)
	}

	ran, err := os.ReadFile(record)
	if err != nil {
		t.Fatalf("go was not run: %v", err)
	}
	if !strings.Contains(string(ran), "args=list -buildvcs=false -deps -json ") {
		t.Errorf("go was run as %q; want go list with -buildvcs=false, which a flag on the command line sets over GOFLAGS", ran)
	}
	if !strings.Contains(string(ran), "toolchain=local\n") {
		t.Errorf("go was run as %q; want GOTOOLCHAIN=local, whatever the caller set", ran)
	}
}
