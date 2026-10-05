package codesignalcli

import (
	"archive/tar"

	"fmt"

	"github.com/lousy-agents/coach/pkg/codesignal"

	"os"

	"path/filepath"

	"strings"
)

// ApplyBaselineSourceScope labels each selected file according to the
// source set it belongs to, same as ApplySourceScope, but instead of
// silently dropping test_only/excluded files it tallies them into excluded,
// grouped by (SourceScope reason, Language) pair, so a Repository Baseline
// report can record what was left out and why. When scope is "all",
// nothing is excluded, matching ApplySourceScope's "all" semantics.
func ApplyBaselineSourceScope(dir, revisionSHA, buildTarget, scope string, files []SelectedFile) (kept []SelectedFile, excluded []codesignal.CoverageGroup, err error) {
	classified, err := classifySourceFiles(dir, revisionSHA, buildTarget, scope, files)
	if err != nil {
		return nil, nil, err
	}
	if scope == "all" {
		return classified, nil, nil
	}

	kept, excluded = tallyClassified(classified)
	return kept, excluded, nil
}
func repositoryRoot(dir string) (string, error) {
	output, err := runGit(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("determining repository root: %w", err)
	}
	root := strings.TrimSpace(output)
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolving repository root: %w", err)
	}
	return resolved, nil
}
func extractTarEntry(dir string, header *tar.Header, reader *tar.Reader) error {
	path, err := safeTarEntryPath(dir, header.Name)
	if err != nil {
		return err
	}
	switch header.Typeflag {
	case tar.TypeXGlobalHeader, tar.TypeXHeader:

		return nil
	case tar.TypeDir:
		return os.MkdirAll(path, os.FileMode(header.Mode))
	case tar.TypeReg:
		return extractTarRegularFile(path, header, reader)
	case tar.TypeSymlink:
		return extractTarSymlink(path, header)
	default:
		return fmt.Errorf("unsupported archive entry %q", header.Name)
	}
}

// skipJSONCBlockComment returns the index of the '/' closing the "/* */"
// comment starting at data[i:i+2]. ok is false when the comment is
// unterminated.
func skipJSONCBlockComment(data []byte, i int) (newIndex int, ok bool) {
	i += 2
	for i+1 < len(data) && !(data[i] == '*' && data[i+1] == '/') {
		i++
	}
	if i+1 >= len(data) {
		return 0, false
	}
	return i + 1, true
}

// ApplySourceScope labels each selected file according to the source set it
// belongs to, then splits out files known not to ship when scope is not
// "all" into excluded, grouped by (SourceScope reason, Language) pair, so
// the diff flow can record what was left out and why. Unknown files are
// deliberately retained in kept so an incomplete project configuration
// cannot silently hide a finding.
func ApplySourceScope(dir, headSHA, buildTarget, scope string, files []SelectedFile) (kept []SelectedFile, excluded []codesignal.CoverageGroup, err error) {
	classified, err := classifySourceFiles(dir, headSHA, buildTarget, scope, files)
	if err != nil {
		return nil, nil, err
	}
	if scope == "all" {
		return classified, nil, nil
	}

	kept, excluded = tallyClassified(classified)
	return kept, excluded, nil
}
