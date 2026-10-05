package codesignalcli

import (
	"archive/tar"

	"io"
	"os"

	"path/filepath"

	"strings"
)

const (
	SourceScopeProduction = "production"
	SourceScopeTestOnly   = "test_only"
	SourceScopeExcluded   = "excluded"
	SourceScopeUnknown    = "unknown"
)

// ApplySourceScope labels each selected file according to the source set it
// belongs to, then splits out files known not to ship when scope is not
// "all" into excluded, grouped by (SourceScope reason, Language) pair, so
// the diff flow can record what was left out and why. Unknown files are
// deliberately retained in kept so an incomplete project configuration
// cannot silently hide a finding.

// ApplyBaselineSourceScope labels each selected file according to the
// source set it belongs to, same as ApplySourceScope, but instead of
// silently dropping test_only/excluded files it tallies them into excluded,
// grouped by (SourceScope reason, Language) pair, so a Repository Baseline
// report can record what was left out and why. When scope is "all",
// nothing is excluded, matching ApplySourceScope's "all" semantics.

// tallyClassified splits classified (files already labeled by
// classifySourceFiles) into files that ship (kept) and files that don't
// (excluded), grouped by (SourceScope reason, Language) pair. It is shared
// by ApplySourceScope and ApplyBaselineSourceScope, whose only difference is
// what they do with the two results.

// classifySourceFiles labels each selected file's SourceScope without
// filtering any of them out, so ApplySourceScope and
// ApplyBaselineSourceScope can share the classification logic while
// applying different policies for what happens to test_only/excluded files.
func classifySourceFiles(dir, headSHA, buildTarget, scope string, files []SelectedFile) ([]SelectedFile, error) {
	if scope == "all" {
		classified := make([]SelectedFile, len(files))
		for i, file := range files {
			file.SourceScope = classifyFilename(file)
			classified[i] = file
		}
		return classified, nil
	}

	repositoryRoot, err := repositoryRoot(dir)
	if err != nil {
		return nil, err
	}

	// Analysis reads only committed objects from headSHA. Build source scope
	// from the same snapshot so local edits cannot affect which findings
	// appear.
	snapshotDir, err := createSnapshot(repositoryRoot, headSHA)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(snapshotDir)

	goProduction, err := goProductionFiles(snapshotDir, repositoryRoot, dir, buildTarget)
	if err != nil {
		return nil, err
	}
	config, hasTSConfig, err := loadTSConfig(snapshotDir)
	if err != nil {
		return nil, err
	}

	classified := make([]SelectedFile, len(files))
	for i, file := range files {
		file.SourceScope = classifySourceFile(file, goProduction, buildTarget, config, hasTSConfig)
		classified[i] = file
	}
	return classified, nil
}

// goProductionFiles returns Go source files selected by the requested target.
// go list applies both dependency reachability and Go build constraints.

func AuthoringRepositoryRoot(dir string) (string, error) {
	return repositoryRoot(dir)
}

// snapshotBuildTarget preserves the meaning of relative package patterns
// supplied from a subdirectory while making them point at the HEAD snapshot.

// Metadata headers are consumed by archive/tar and do not represent
// filesystem entries in the snapshot.

// safeTarEntryPath joins name onto dir and rejects any result that would
// escape dir (a path-traversal entry such as "../../etc/passwd").

func extractTarRegularFile(path string, header *tar.Header, reader io.Reader) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(file, reader)
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

type tsConfig struct {
	// Path-only; npm package extends are not resolved. Non-string (e.g. TS 5
	// multi-base arrays) is ignored so the rest of the file still parses.
	Extends string   `json:"extends"`
	Include []string `json:"include"`
	Exclude []string `json:"exclude"`
	// Non-nil distinguishes explicit "files": [] (selects nothing) from omitted.
	Files *[]string `json:"files"`
}

// UnmarshalJSON accepts only string extends; other shapes leave Extends empty
// without failing the whole config.

// loadTSConfig resolves path-shaped extends chains. Child fields override
// base fields (no merge). Inherited patterns are rebased to the declaring
// base's directory. Cycles, escapes, npm extends, and I/O failures fail open
// (ok=false) — tsconfig is attacker-influenced (e.g. fork PR input).

// EvalSymlinks so rebase math uses the same path space as baseDir.

// applyTSConfigBase fills whichever of config's Include/Exclude/Files the
// child left unset with base's own, rebased to base's directory.

// resolveExtendedTSConfig joins extends relative to dir, then enforces the
// snapshotRoot boundary after EvalSymlinks (extractTar preserves symlinks;
// a lexical-only check would read through an in-bounds symlink to a host
// path). Boundary is snapshotRoot, not the current hop's directory.

// resolveTSConfigExtendsTarget retries with ".json" when the literal path is missing.

// rebaseTSConfigPatterns prefixes patterns with baseDir relative to snapshotRoot
// (TS resolves them against the declaring config's directory). Nil stays nil.

// isTSConfigPathSpecifier is true for ./ ../ .\ ..\ or absolute paths only.
func isTSConfigPathSpecifier(extends string) bool {
	return strings.HasPrefix(extends, "./") ||
		strings.HasPrefix(extends, "../") ||
		strings.HasPrefix(extends, `.\`) ||
		strings.HasPrefix(extends, `..\`) ||
		filepath.IsAbs(extends)
}

// readTSConfigFile does not follow extends. Missing/malformed => ok=false;
// other I/O errors are returned.

// stripJSONCComments strips // and /* */ outside strings, then trailing commas.
// Unterminated /* returns the original bytes so Unmarshal fails closed.

// advanceInsideJSONString writes b (already known to be inside a JSON
// string literal) to out and returns the string/escape state after
// consuming it. Shared by stripJSONCComments and stripTrailingCommas so
// neither strips a comment- or comma-like byte that only appears inside a
// string value.

// skipJSONCLineComment returns the index of the '\n' terminating the "//"
// comment starting at data[i], or len(data) if it runs to EOF.

// skipJSONCBlockComment returns the index of the '/' closing the "/* */"
// comment starting at data[i:i+2]. ok is false when the comment is
// unterminated.

// trailingCommaFollowedByClose reports whether the comma at data[i] is
// followed only by whitespace before a closing '}' or ']', making it a
// JSONC trailing comma to drop rather than emit.

// matchesInclude is the union of files and include (TS semantics). Match-all
// only when both are absent; explicit empty files selects nothing.

func (c tsConfig) matchesExclude(path string) bool { return matchesAny(path, c.Exclude) }
