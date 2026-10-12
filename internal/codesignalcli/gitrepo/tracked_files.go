package gitrepo

import (
	"bytes"
	"fmt"
	"path/filepath"
	"sort"

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
	output, err := RunBytes(dir, "ls-tree", "-r", "-z", "--name-only", revisionSHA)
	if err != nil {
		return nil, codesignal.Coverage{}, &OperationalError{Message: fmt.Sprintf("coach codesignal: git ls-tree failed: %s", err)}
	}

	var coverage codesignal.Coverage
	var files []SelectedFile
	unsupportedCounts := make(map[string]int)

	for _, path := range SplitNULPaths(output) {
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

// SplitNULPaths splits the NUL-delimited output of a git command like
// `ls-tree -z --name-only` into individual paths. It never invokes a shell
// and never interprets path bytes beyond splitting on NUL, so paths
// containing spaces, quotes, newlines, or non-ASCII bytes round-trip
// exactly (mirroring parseNameStatusZ's approach for the same reason).
func SplitNULPaths(data []byte) []string {
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

// coverageLanguageLabel returns a stable, non-empty label for a
// CoverageGroup.Language so an extensionless file (LICENSE, Makefile;
// filepath.Ext returns "") never produces an empty, JSON-omitted label that
// renders as a blank in text output.
func coverageLanguageLabel(ext string) string {
	if ext == "" {
		return "(no extension)"
	}
	return ext
}
