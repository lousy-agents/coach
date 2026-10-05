package codesignalcli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

const (
	SourceScopeProduction = "production"
	SourceScopeTestOnly   = "test_only"
	SourceScopeExcluded   = "excluded"
	SourceScopeUnknown    = "unknown"
)

// classifySourceFiles labels each selected file's SourceScope without
// filtering any of them out, so ApplySourceScope and
// ApplyBaselineSourceScope can share the classification logic while
// applying different policies for what happens to test_only/excluded files.
func classifySourceFiles(dir, headSHA, buildTarget, scope string, files []gitrepo.SelectedFile) ([]gitrepo.SelectedFile, error) {
	if scope == "all" {
		classified := make([]gitrepo.SelectedFile, len(files))
		for i, file := range files {
			file.SourceScope = classifyFilename(file)
			classified[i] = file
		}
		return classified, nil
	}

	repositoryRoot, err := gitrepo.RepositoryRoot(dir)
	if err != nil {
		return nil, err
	}

	// Analysis reads only committed objects from headSHA. Build source scope
	// from the same snapshot so local edits cannot affect which findings
	// appear.
	snapshotDir, err := gitrepo.ExtractRevision(repositoryRoot, headSHA)
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

	classified := make([]gitrepo.SelectedFile, len(files))
	for i, file := range files {
		file.SourceScope = classifySourceFile(file, goProduction, buildTarget, config, hasTSConfig)
		classified[i] = file
	}
	return classified, nil
}

func AuthoringRepositoryRoot(dir string) (string, error) {
	return gitrepo.RepositoryRoot(dir)
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

// EvalSymlinks so rebase math uses the same path space as baseDir.
// isTSConfigPathSpecifier is true for ./ ../ .\ ..\ or absolute paths only.
func isTSConfigPathSpecifier(extends string) bool {
	return strings.HasPrefix(extends, "./") ||
		strings.HasPrefix(extends, "../") ||
		strings.HasPrefix(extends, `.\`) ||
		strings.HasPrefix(extends, `..\`) ||
		filepath.IsAbs(extends)
}

func (c tsConfig) matchesExclude(path string) bool { return matchesAny(path, c.Exclude) }

// matchesInclude is the union of files and include (TS semantics). Match-all
// only when both are absent; explicit empty files selects nothing.
func (c tsConfig) matchesInclude(path string) bool {
	if c.Files != nil && matchesAny(path, *c.Files) {
		return true
	}
	if len(c.Include) > 0 {
		return matchesAny(path, c.Include)
	}
	return c.Files == nil
}

// UnmarshalJSON accepts only string extends; other shapes leave Extends empty
// without failing the whole config.
func (c *tsConfig) UnmarshalJSON(data []byte) error {
	type tsConfigAlias tsConfig
	var aux struct {
		Extends json.RawMessage `json:"extends"`
		tsConfigAlias
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*c = tsConfig(aux.tsConfigAlias)
	if len(aux.Extends) > 0 {
		var extends string
		if err := json.Unmarshal(aux.Extends, &extends); err == nil {
			c.Extends = extends
		}
	}
	return nil
}
