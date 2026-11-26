package evalharness

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// provisionCompiler gives a copy of a TypeScript program the pinned compiler the way an install would: the
// typescript package and its platform package under node_modules, which the copy's .gitignore keeps out of the commit.
func provisionCompiler(tsc, work string) error {
	resolved, err := filepath.EvalSymlinks(tsc)
	if err != nil {
		return err
	}
	packageDir := filepath.Dir(filepath.Dir(resolved))
	installed := filepath.Dir(packageDir)
	target := filepath.Join(work, "node_modules")
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}
	for _, name := range []string{filepath.Base(packageDir), "@typescript"} {
		if output, err := exec.Command("cp", "-R", filepath.Join(installed, name), target).CombinedOutput(); err != nil {
			return fmt.Errorf("copying %s into the program: %v: %s", name, err, output)
		}
	}
	return os.WriteFile(filepath.Join(work, ".gitignore"), []byte("node_modules/\n"), 0o644)
}
