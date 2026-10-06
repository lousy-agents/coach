// Package sourcelayout enforces ADR-007's naming rule: a Go file and the
// identifiers it declares are named for their responsibility, never for the
// position they were cut from (main_part2.go, body_mainPart2Test_44) or a
// generated hash (sigstartRecordingProxyListener39957725).
package sourcelayout

import (
	"io/fs"
	"path/filepath"
	"strings"
)

// Violation is one file that breaks the naming rule. Path is slash-separated
// and relative to the checked root.
type Violation struct {
	Path   string
	Reason string
}

// testdata holds fixtures that are inputs to analysis, not this repository's
// source, so their names are chosen by the scenario they model.
var skipDirNames = map[string]struct{}{
	"vendor":       {},
	"node_modules": {},
	"testdata":     {},
}

// Check walks root and reports every Go file whose name or identifiers are
// positional or hashed.
func Check(root string) ([]Violation, error) {
	var violations []Violation
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && path != root {
			return skipDir(d.Name())
		}
		if !strings.HasSuffix(d.Name(), ".go") {
			return nil
		}
		reason, err := fileViolation(path, d.Name())
		if err != nil || reason == "" {
			return err
		}
		violations = append(violations, Violation{Path: relativeSlashPath(root, path), Reason: reason})
		return nil
	})
	return violations, err
}

// Directories the go tool ignores (leading "." or "_") are not built as this
// module's source; that includes agent worktrees under .claude/worktrees.
func skipDir(name string) error {
	_, skip := skipDirNames[name]
	if skip || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
		return fs.SkipDir
	}
	return nil
}

func relativeSlashPath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

func fileViolation(path, name string) (string, error) {
	if reason := fileNameViolation(name); reason != "" {
		return reason, nil
	}
	return identifierViolation(path)
}
