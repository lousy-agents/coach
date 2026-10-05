package gitrepo

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

// SelectedFile identifies one file changed between a merge-base and HEAD
// that is eligible for CodeSignal analysis.
type SelectedFile struct {
	Path        string
	Status      codesignal.ChangeStatus
	Language    semantics.Language
	SourceScope string
}

// SelectChangedFiles diffs mergeBaseSHA against HEAD in dir and returns the
// files eligible for analysis, plus diagnostics for changes that are out of
// scope (unsupported statuses such as T/U/X/B, unsupported languages).
// Rename and copy new paths are selected so HEAD content is analyzed.
// They are not marked added: that would inherit #262's introduced
// lifecycle. Continuity against the old path is out of scope, so each
// selected R/C path also gets a continuity_not_determined diagnostic.
func SelectChangedFiles(dir, mergeBaseSHA string) ([]SelectedFile, []codesignal.Diagnostic, error) {
	// --find-renames uses git's default 50% threshold. --find-copies-harder
	// also considers unmodified files as copy sources, which is more
	// aggressive than git's defaults; the extra R/C records are analyzed
	// rather than dropped.
	output, err := RunBytes(dir, "diff", "--name-status", "-z", "--find-renames", "--find-copies-harder", mergeBaseSHA, "HEAD")
	if err != nil {
		return nil, nil, &OperationalError{Message: fmt.Sprintf("coach codesignal: git diff failed: %s", err)}
	}

	records, err := parseNameStatusZ(output)
	if err != nil {
		return nil, nil, &OperationalError{Message: fmt.Sprintf("coach codesignal: %s", err)}
	}

	var selected []SelectedFile
	var diagnostics []codesignal.Diagnostic

	for _, record := range records {
		switch {
		case strings.HasPrefix(record.status, "R") || strings.HasPrefix(record.status, "C"):
			path := record.paths[len(record.paths)-1]
			sf, diag, ok := selectSupportedPath(path, "")
			if !ok {
				diagnostics = append(diagnostics, diag)
				continue
			}
			selected = append(selected, sf)
			diagnostics = append(diagnostics, codesignal.Diagnostic{
				Kind:    "continuity_not_determined",
				Path:    path,
				Message: "rename/copy lifecycle continuity was not determined",
			})
		case record.status == "A" || record.status == "M" || record.status == "D":
			sf, diag, ok := selectSupportedPath(record.paths[0], statusToChangeStatus(record.status))
			if !ok {
				diagnostics = append(diagnostics, diag)
				continue
			}
			selected = append(selected, sf)
		default:
			diagnostics = append(diagnostics, codesignal.Diagnostic{
				Kind:    "unsupported_change_type",
				Path:    record.paths[0],
				Message: fmt.Sprintf("change status %q is not supported", record.status),
			})
		}
	}

	return selected, diagnostics, nil
}

func statusToChangeStatus(status string) codesignal.ChangeStatus {
	switch status {
	case "A":
		return "added"
	case "M":
		return "modified"
	case "D":
		return "removed"
	default:
		return ""
	}
}

func selectSupportedPath(path string, status codesignal.ChangeStatus) (SelectedFile, codesignal.Diagnostic, bool) {
	lang, ok := semantics.LanguageForExtension(filepath.Ext(path))
	if !ok {
		return SelectedFile{}, codesignal.Diagnostic{
			Kind:    "unsupported_language",
			Path:    path,
			Message: fmt.Sprintf("file extension %q is not a supported language", filepath.Ext(path)),
		}, false
	}
	return SelectedFile{Path: path, Status: status, Language: lang}, codesignal.Diagnostic{}, true
}
