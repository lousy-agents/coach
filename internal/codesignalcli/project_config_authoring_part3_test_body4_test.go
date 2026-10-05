package codesignalcli

import (
	"bytes"

	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func body_projectConfigAuthoringPart3Test_outputPathSParentIsASymlinkTheWriteIsRefusedAndN_201(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	dir := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dir, "link")); err != nil {
		t.Fatalf("failed to prepare a symlinked parent: %v", err)
	}
	outputPath := "link/project.json"
	out := &bytes.Buffer{}
	in := strings.NewReader(strings.Join([]string{
		"1",
		"domain", "internal/domain",
		"",
		"",
		"",
		"approve",
	}, "\n") + "\n")

	result := AuthorProjectConfig(dir, in, out, out, discovered, outputPath, true)

	if !result.Approved {
		t.Fatalf("expected Approved = true, got false; output:\n%s", out.String())
	}

	if result.WriteError == nil || !strings.Contains(result.WriteError.Error(), `parent path component "link" is a symlink`) {
		t.Fatalf("expected WriteError to report the symlinked-parent rejection, got %v", result.WriteError)
	}
	if result.OutputExists {
		t.Fatalf("expected OutputExists = false for a rejected symlinked-parent path")
	}

	if _, err := os.Stat(filepath.Join(outside, "project.json")); !os.IsNotExist(err) {
		t.Fatalf("expected nothing to be written at %q outside the repository root, stat err = %v", filepath.Join(outside, "project.json"), err)
	}
}
