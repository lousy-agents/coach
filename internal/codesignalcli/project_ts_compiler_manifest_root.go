package codesignalcli

import (
	"os"
	"path/filepath"
	"strings"
)

func resolveProjectRoot(worktreeRoot, root string) projectRootOutcome {
	outcome := projectRootOutcome{root: root}
	manifestDir, ok := nearestPackageJSONDir(selectedRootAbs(worktreeRoot, root), worktreeRoot)
	if !ok {
		return outcome
	}
	outcome.manifestDir = manifestDir

	manifestFields, manifestUnreadable := readPackageJSONTypescriptFields(manifestDir)
	if manifestUnreadable {
		outcome.unreadable = true
		return outcome
	}
	installedVersion, installedExists, installedUnreadable := readInstalledTypescriptVersion(manifestDir)
	if installedUnreadable {
		outcome.unreadable = true
		return outcome
	}

	declaration, ambiguous := declaredTypescriptVersion(manifestFields)
	if ambiguous {
		outcome.ambiguous = true
		return outcome
	}
	// The installed compiler is this root's candidate, classed by its probed
	// version alone; the declaration governs setup choices and warnings only
	// (epic #280, owner decision D4). A declaration with nothing installed
	// beside it therefore resolves nothing, whatever it declares.
	outcome.declaration = declaration
	outcome.installed = installedExists
	if installedExists {
		outcome.finding = installedVersion
		outcome.candidate = installedVersion
	}
	outcome.disqualified = disqualifyingDeclaration(declaration)
	return outcome
}

func selectedRootAbs(worktreeRoot, root string) string {
	if root == "" || root == "." {
		return worktreeRoot
	}
	return filepath.Join(worktreeRoot, filepath.FromSlash(root))
}

func nearestPackageJSONDir(start, ceiling string) (string, bool) {
	start = filepath.Clean(start)
	ceiling = filepath.Clean(ceiling)
	rel, err := filepath.Rel(ceiling, start)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", false
	}

	current := start
	for {
		info, statErr := os.Stat(filepath.Join(current, "package.json"))
		if statErr == nil && !info.IsDir() {
			return current, true
		}
		if current == ceiling {
			return "", false
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", false
		}
		current = parent
	}
}
