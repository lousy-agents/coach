package sourcescope

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// goProductionFiles returns Go source files selected by the requested target.
// go list applies both dependency reachability and Go build constraints.
func goProductionFiles(snapshotDir, repositoryRoot, invocationDir, buildTarget string) (map[string]bool, error) {
	if buildTarget == "" {
		return nil, nil
	}
	target, err := snapshotBuildTarget(buildTarget, repositoryRoot, invocationDir, snapshotDir)
	if err != nil {
		return nil, err
	}
	output, err := runCommand(snapshotDir, "go", "list", "-deps", "-json", target)
	if err != nil {
		return nil, fmt.Errorf("determining Go production files for %q: %w", buildTarget, err)
	}

	listed := &listedGoFiles{root: snapshotDir, files: make(map[string]bool)}
	decoder := json.NewDecoder(bytes.NewReader(output))
	for decoder.More() {
		var pkg struct {
			Dir      string
			GoFiles  []string
			CgoFiles []string
		}
		if err := decoder.Decode(&pkg); err != nil {
			return nil, fmt.Errorf("decoding go list output: %w", err)
		}
		listed.add(pkg.Dir, append(pkg.GoFiles, pkg.CgoFiles...))
	}
	return listed.files, nil
}

type listedGoFiles struct {
	root  string
	files map[string]bool
}

func (l *listedGoFiles) add(dir string, names []string) {
	for _, name := range names {
		path, err := filepath.Rel(l.root, filepath.Join(dir, name))
		if err == nil && !strings.HasPrefix(path, ".."+string(filepath.Separator)) {
			l.files[filepath.ToSlash(path)] = true
		}
	}
}
