package configauthoring

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func TestAuthorProjectConfig_DeclinedApprovalNeverReachesTheWritePath(t *testing.T) {
	discovered := projectmodel.TSRootDiscoveryResult{Roots: []string{"apps/api"}, Complete: true}

	t.Run("with --output set, no file is created", func(t *testing.T) {
		withOutputSetNoFileCreated(t, discovered)
	})

	t.Run("with --output unset, nothing beyond the interactive text is written to out", func(t *testing.T) {
		withOutputUnsetNothingBeyondInteractiveTextWritten(t, discovered)
	})
}

func withOutputSetNoFileCreated(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
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

func withOutputUnsetNothingBeyondInteractiveTextWritten(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
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
