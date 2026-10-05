package codesignalcli

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

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

	result := AuthorProjectConfig(dir, in, out, out, discovered, outputPath, true)

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

	result := AuthorProjectConfig(t.TempDir(), in, out, out, discovered, "", false)

	if result.Approved {
		t.Fatalf("expected Approved = false, got true")
	}
	if tail := tailAfterLastPrompt(out.String()); tail != "" {
		t.Fatalf("expected nothing written after the approval prompt for a declined approval, got %q", tail)
	}
}

func body_projectConfigAuthoringPart9Test_108(t *testing.T, tc struct {
	name       string
	discovered projectmodel.TSRootDiscoveryResult
	answer     string
}) {
	out := &bytes.Buffer{}
	in := bufio.NewReader(strings.NewReader(tc.answer))

	promptForRoots(out, in, tc.discovered)

	printed := strings.ToLower(out.String())
	if strings.Contains(printed, "layer") {
		t.Fatalf("root-selection prompt must never mention layers, got output:\n%s", out.String())
	}
	forbiddenGroupings := []string{"boundary", "grouped under", "suggested layer"}
	for _, phrase := range forbiddenGroupings {
		if strings.Contains(printed, phrase) {
			t.Fatalf("root-selection prompt must never propose a layer boundary (found %q), got output:\n%s", phrase, out.String())
		}
	}
}
