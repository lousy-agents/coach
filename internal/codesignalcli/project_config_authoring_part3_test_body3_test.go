package codesignalcli

import (
	"bytes"

	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func body_projectConfigAuthoringPart3Test_outputPathContainsAGitComponentTheWriteIsRefused_136(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	dir := t.TempDir()

	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatalf("failed to prepare a real .git directory: %v", err)
	}
	outputPath := ".git/config.json"
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
	if result.WriteError == nil || !strings.Contains(result.WriteError.Error(), `must not contain a ".git" path component`) {
		t.Fatalf("expected WriteError to report the .git-component rejection, got %v", result.WriteError)
	}
	if result.OutputExists {
		t.Fatalf("expected OutputExists = false for a rejected .git-component path")
	}

	if _, err := os.Stat(filepath.Join(dir, outputPath)); !os.IsNotExist(err) {
		t.Fatalf("expected nothing to be written at %q, stat err = %v", outputPath, err)
	}
}

func body_projectConfigAuthoringPart3Test_outputPathSParentDirectoryDoesNotExistTheWriteIs_170(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	dir := t.TempDir()
	outputPath := "missing-parent/project.json"
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

	if result.WriteError == nil || !strings.Contains(result.WriteError.Error(), `parent directory "missing-parent" does not exist`) {
		t.Fatalf("expected WriteError to report the missing-parent-directory rejection, got %v", result.WriteError)
	}
	if result.OutputExists {
		t.Fatalf("expected OutputExists = false for a rejected missing-parent path")
	}

	if _, err := os.Stat(filepath.Join(dir, outputPath)); !os.IsNotExist(err) {
		t.Fatalf("expected nothing to be written at %q, stat err = %v", outputPath, err)
	}
}
