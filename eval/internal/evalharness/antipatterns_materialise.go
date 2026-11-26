package evalharness

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// materialise copies a variant into dir, reading a source stored with a .txt suffix (main.go.txt, go.mod.txt,
// .lousy-agents/coach.json.txt) as the file itself, so that a deliberately bad variant is not Go code in the repository that holds it.
func materialise(from, to string) error {
	return filepath.WalkDir(from, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(from, path)
		target := filepath.Join(to, strippedSuffix(rel))
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, content, 0o644)
	})
}

func strippedSuffix(rel string) string {
	base, found := strings.CutSuffix(rel, ".txt")
	if found && (strings.HasSuffix(base, ".go") || base == "go.mod" || base == "go.sum" || base == policyPath) {
		return base
	}
	return rel
}

// hostIndependentGit keeps the host's git configuration, and any hook it names, out of the commit that makes a fixture scannable.
var hostIndependentGit = []string{"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1"}

func commitAll(dir string) error {
	for _, step := range [][]string{
		{"init", "-q"}, {"add", "-A"},
		{"-c", "user.name=coach-eval", "-c", "user.email=coach-eval@example.invalid", "commit", "-q", "-m", "fixture"},
	} {
		if output, err := runIn(dir, hostIndependentGit, "", "git", step...); err != nil {
			return fmt.Errorf("git %s: %v: %s", step[0], err, output)
		}
	}
	return nil
}
