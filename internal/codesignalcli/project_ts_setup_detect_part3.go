package codesignalcli

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// scanNpmrcLines reads root's committed .npmrc (the ini-style config file
// both npm and pnpm honor -- verified empirically that `pnpm config get`
// resolves a committed .npmrc's keys the same way npm does) and calls handle
// with each non-comment, non-blank line's lowercased key and unquoted value,
// stopping at the first non-empty detail handle returns. Absence of .npmrc
// is safe ("" with handle never called); a .npmrc that exists but cannot be
// read (permission denied, a directory, a dangling symlink) is a hazard in
// its own right, distinct from no .npmrc existing at all -- fail-closed
// rather than treating an unreadable hazard file as absent.
func scanNpmrcLines(root string, handle func(key, value string) string) string {
	path := filepath.Join(root, ".npmrc")
	data, err := os.ReadFile(path)
	if err != nil {
		if _, statErr := os.Lstat(path); errors.Is(statErr, fs.ErrNotExist) {
			return ""
		}
		return "committed .npmrc could not be read"
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = trimConfigValueQuotes(strings.TrimSpace(value))
		if detail := handle(key, value); detail != "" {
			return detail
		}
	}
	return ""
}

// packageManagerContexts resolves the directories checkPackageManager reads
// manager metadata from: each selected root's nearest package.json, which is
// exactly the context resolveCompiler resolves that root's compiler against
// (resolveProjectRoot). A root with no manifest at or above it contributes no
// context; when no root contributes one, the worktree root stands in, so a
// repository whose only metadata is a top-level lockfile beside no
// package.json is still classified rather than silently unchecked --
// fellBack reports when that stand-in fired, so a caller without a
// validated policy can tell that classification apart from one resolveCompiler
// actually found a manifest for (checkPackageManager's own R1 gate).
func packageManagerContexts(worktreeRoot string, roots []string) (contexts []string, fellBack bool) {
	if len(roots) == 0 {
		roots = []string{"."}
	}
	contexts = make([]string, 0, len(roots))
	for _, root := range roots {
		if manifestDir, ok := nearestPackageJSONDir(selectedRootAbs(worktreeRoot, root), worktreeRoot); ok {
			contexts = append(contexts, manifestDir)
		}
	}
	if len(contexts) == 0 {
		return []string{worktreeRoot}, true
	}
	return dedupeStrings(contexts), false
}
