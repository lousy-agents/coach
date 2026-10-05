package codesignalcli

import (
	"encoding/json"
	"fmt"

	"os"

	"path/filepath"

	"strings"
)

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
func createSnapshot(repositoryRoot, revision string) (string, error) {
	archive, err := runGitBytes(repositoryRoot, "archive", "--format=tar", revision)
	if err != nil {
		return "", fmt.Errorf("reading source snapshot %q: %w", revision, err)
	}
	dir, err := os.MkdirTemp("", "coach-codesignal-snapshot-*")
	if err != nil {
		return "", fmt.Errorf("creating source snapshot: %w", err)
	}
	if err := extractTar(dir, archive); err != nil {
		os.RemoveAll(dir)
		return "", fmt.Errorf("extracting source snapshot: %w", err)
	}
	return dir, nil
}

// applyTSConfigBase fills whichever of config's Include/Exclude/Files the
// child left unset with base's own, rebased to base's directory.
func applyTSConfigBase(config tsConfig, snapshotRoot, baseDir string, base tsConfig) tsConfig {
	if config.Include == nil {
		config.Include = rebaseTSConfigPatterns(snapshotRoot, baseDir, base.Include)
	}
	if config.Exclude == nil {
		config.Exclude = rebaseTSConfigPatterns(snapshotRoot, baseDir, base.Exclude)
	}
	if config.Files == nil && base.Files != nil {
		rebased := rebaseTSConfigPatterns(snapshotRoot, baseDir, *base.Files)
		config.Files = &rebased
	}
	return config
}

// safeTarEntryPath joins name onto dir and rejects any result that would
// escape dir (a path-traversal entry such as "../../etc/passwd").
func safeTarEntryPath(dir, name string) (string, error) {
	path := filepath.Join(dir, filepath.FromSlash(name))
	rel, err := filepath.Rel(dir, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe archive path %q", name)
	}
	return path, nil
}
