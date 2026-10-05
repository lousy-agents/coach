package codesignalcli

import (
	"reflect"
	"testing"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestSelectChangedFilesRenameSelectsNewPathWithoutAddedStatus(t *testing.T) {
	dir := newTempGitRepoT(t)
	initialSHA := commitFileT(t, dir, "old.go", "package old\n// padding so rename detection kicks in\n// more padding\n// more padding\n// more padding\n")
	headSHA := renameFileT(t, dir, "old.go", "new.go")

	selected, diagnostics, continuity, err := SelectChangedFiles(dir, initialSHA, headSHA)
	if err != nil {
		t.Fatalf("SelectChangedFiles: unexpected error: %v", err)
	}
	if want := []codesignal.PathContinuity{{Path: "new.go", PreviousPath: "old.go"}}; !reflect.DeepEqual(continuity, want) {
		t.Errorf("continuity = %#v, want %#v", continuity, want)
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
			if d.Side != "head" {
				t.Errorf("continuity diagnostic Side = %q, want %q (continuity detection is head-side only)", d.Side, "head")
			}
			if d.Revision != headSHA {
				t.Errorf("continuity diagnostic Revision = %q, want %q", d.Revision, headSHA)
			}
		}
	}
	if !foundContinuity {
		t.Errorf("diagnostics = %#v, want continuity_not_determined on new.go", diagnostics)
	}
}

func TestSelectChangedFilesLanguageFiltering(t *testing.T) {
	dir := newTempGitRepoT(t)
	initialSHA := commitFileT(t, dir, "keep.go", "package keep\n")
	commitFileT(t, dir, "keep.go", "package keep\n\nfunc F() {}\n")
	headSHA := commitFileT(t, dir, "unsupported.txt", "plain text\n")

	selected, diagnostics, continuity, err := SelectChangedFiles(dir, initialSHA, headSHA)
	if err != nil {
		t.Fatalf("SelectChangedFiles: unexpected error: %v", err)
	}
	if len(continuity) != 0 {
		t.Errorf("continuity = %#v, want none without a rename or copy", continuity)
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

// Project analysis reads files the per-file selection rejects, so an
// unsupported-language rename still has to report its pair.
func TestSelectChangedFilesReportsContinuityForUnsupportedLanguageRename(t *testing.T) {
	dir := newTempGitRepoT(t)
	initialSHA := commitFileT(t, dir, "old.js", "export const a = 1;\n// padding\n// more padding\n// more padding\n")
	headSHA := renameFileT(t, dir, "old.js", "new.js")

	selected, _, continuity, err := SelectChangedFiles(dir, initialSHA, headSHA)
	if err != nil {
		t.Fatalf("SelectChangedFiles: unexpected error: %v", err)
	}
	if len(selected) != 0 {
		t.Errorf("selected = %#v, want no per-file analysis of an unsupported language", selected)
	}
	if want := []codesignal.PathContinuity{{Path: "new.js", PreviousPath: "old.js"}}; !reflect.DeepEqual(continuity, want) {
		t.Errorf("continuity = %#v, want %#v", continuity, want)
	}
}
