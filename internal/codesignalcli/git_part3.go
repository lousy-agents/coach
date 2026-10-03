package codesignalcli

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

// DiscoverTrackedFiles lists every file tracked by Git at revisionSHA (via
// `git ls-tree -r -z --name-only`), independent of any diff or history --
// this is what lets a Repository Baseline scan see a file that was
// committed once and never touched again, which a diff against that same
// revision would never surface. Unsupported-language files are not turned
// into per-file diagnostics (that would flood the report with one entry per
// file in a large tree); instead they are tallied into
// coverage.Unsupported, grouped by extension. A git failure (bad revision,
// missing git executable, etc.) is returned as an *OperationalError.
func DiscoverTrackedFiles(dir, revisionSHA string) ([]SelectedFile, codesignal.Coverage, error) {
	output, err := runGitBytes(dir, "ls-tree", "-r", "-z", "--name-only", revisionSHA)
	if err != nil {
		return nil, codesignal.Coverage{}, &OperationalError{Message: fmt.Sprintf("coach codesignal: git ls-tree failed: %s", err)}
	}

	var coverage codesignal.Coverage
	var files []SelectedFile
	unsupportedCounts := make(map[string]int)

	for _, path := range splitNULPaths(output) {
		coverage.TrackedFilesDiscovered++

		ext := filepath.Ext(path)
		lang, ok := semantics.LanguageForExtension(ext)
		if !ok {
			unsupportedCounts[coverageLanguageLabel(ext)]++
			continue
		}

		files = append(files, SelectedFile{Path: path, Language: lang})
	}

	extensions := make([]string, 0, len(unsupportedCounts))
	for ext := range unsupportedCounts {
		extensions = append(extensions, ext)
	}
	sort.Strings(extensions)
	for _, ext := range extensions {
		coverage.Unsupported = append(coverage.Unsupported, codesignal.CoverageGroup{
			Reason:   "unsupported_language",
			Language: ext,
			Count:    unsupportedCounts[ext],
		})
	}

	return files, coverage, nil
}

// splitNULPaths splits the NUL-delimited output of a git command like
// `ls-tree -z --name-only` into individual paths. It never invokes a shell
// and never interprets path bytes beyond splitting on NUL, so paths
// containing spaces, quotes, newlines, or non-ASCII bytes round-trip
// exactly (mirroring parseNameStatusZ's approach for the same reason).
func splitNULPaths(data []byte) []string {
	fields := bytes.Split(bytes.TrimSuffix(data, []byte{0}), []byte{0})
	if len(fields) == 1 && len(fields[0]) == 0 {
		return nil
	}
	paths := make([]string, len(fields))
	for i, field := range fields {
		paths[i] = string(field)
	}
	return paths
}
func runGitBytes(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("%s: %s", err, strings.TrimSpace(stderr.String()))
		}
		return nil, err
	}
	return stdout.Bytes(), nil
}

// Reason reports e's failure description without any interpolated
// absolute path. See the reason field's doc comment for why this exists.
func (e *OperationalError) Reason() string {
	if e.reason != "" {
		return e.reason
	}
	return e.Message
}
