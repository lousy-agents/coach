package evalharness

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// evalModulePath is the module whose tool directive pins gocognit.
const evalModulePath = "module github.com/lousy-agents/coach/eval"

var errEvalModuleNotFound = errors.New("the eval module is not above the working directory; set EVAL_MODULE_DIR to it")

// findEvalModule returns the directory of the eval module: EVAL_MODULE_DIR, else the nearest directory at or
// above the working directory that holds it, or an eval directory beside it.
func findEvalModule() (string, error) {
	if dir := os.Getenv("EVAL_MODULE_DIR"); dir != "" {
		return dir, nil
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if found, ok := moduleAt(dir); ok {
			return found, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errEvalModuleNotFound
		}
		dir = parent
	}
}

func moduleAt(dir string) (string, bool) {
	for _, candidate := range []string{dir, filepath.Join(dir, "eval")} {
		if declaresEvalModule(filepath.Join(candidate, "go.mod")) {
			return candidate, true
		}
	}
	return "", false
}

func declaresEvalModule(goMod string) bool {
	content, err := os.ReadFile(goMod)
	return err == nil && strings.Contains(string(content), evalModulePath+"\n")
}
