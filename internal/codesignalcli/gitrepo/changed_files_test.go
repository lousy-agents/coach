package gitrepo

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestSelectChangedFilesRenameSelectsNewPathWithoutAddedStatus(t *testing.T) {
	dir := gitfixture.Init(t)
	initialSHA := gitfixture.CommitFile(t, dir, "old.go", "package old\n// padding so rename detection kicks in\n// more padding\n// more padding\n// more padding\n")
	gitfixture.Rename(t, dir, "old.go", "new.go")

	selected, diagnostics, err := SelectChangedFiles(dir, initialSHA)
	if err != nil {
		t.Fatalf("SelectChangedFiles: unexpected error: %v", err)
	}

	if len(selected) != 1 {
		t.Fatalf("selected = %#v, want the rename's new path", selected)
	}
	if selected[0].Path != "new.go" || selected[0].Status == "added" || selected[0].Language != semantics.LanguageGo {
		t.Errorf("selected[0] = %#v, want new.go with a non-added status and language go", selected[0])
	}

	foundContinuity := false
	for _, d := range diagnostics {
		if d.Kind == "unsupported_change_type" && d.Path == "new.go" {
			t.Errorf("diagnostics = %#v, rename new path must not be skipped as unsupported_change_type", diagnostics)
		}
		if d.Kind == "continuity_not_determined" && d.Path == "new.go" {
			foundContinuity = true
		}
	}
	if !foundContinuity {
		t.Errorf("diagnostics = %#v, want continuity_not_determined on new.go", diagnostics)
	}
}

func TestSelectChangedFilesLanguageFiltering(t *testing.T) {
	dir := gitfixture.Init(t)
	initialSHA := gitfixture.CommitFile(t, dir, "keep.go", "package keep\n")
	gitfixture.CommitFile(t, dir, "keep.go", "package keep\n\nfunc F() {}\n")
	gitfixture.CommitFile(t, dir, "unsupported.txt", "plain text\n")

	selected, diagnostics, err := SelectChangedFiles(dir, initialSHA)
	if err != nil {
		t.Fatalf("SelectChangedFiles: unexpected error: %v", err)
	}

	if len(selected) != 1 {
		t.Fatalf("selected = %#v, want exactly one supported-language file", selected)
	}
	if selected[0].Path != "keep.go" || selected[0].Language != semantics.LanguageGo || selected[0].Status != "modified" {
		t.Errorf("selected[0] = %#v, want keep.go/modified/go", selected[0])
	}

	if len(diagnostics) != 1 || diagnostics[0].Kind != "unsupported_language" || diagnostics[0].Path != "unsupported.txt" {
		t.Errorf("diagnostics = %#v, want one unsupported_language diagnostic for unsupported.txt", diagnostics)
	}
}
