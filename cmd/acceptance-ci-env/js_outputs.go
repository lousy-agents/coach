package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// jsInstalled is what the ts-project-backend specs need: CI's job installs it
// with `mise run js-install` before it runs them, and git ignores it, so the
// repository copy lacks it.
const jsInstalled = "js/semantics/node_modules"

// positiveTSProjectBackend matches the label as a term a label filter selects,
// not one it negates with `!`.
var positiveTSProjectBackend = regexp.MustCompile(`(^|[^!\w-])ts-project-backend\b`)

// labelFilters returns the values of every -ginkgo.label-filter flag, in
// either the `=value` or the two-argument form.
func labelFilters(args []string) []string {
	var values []string
	for i, arg := range args {
		name, value, hasValue := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		if name != "ginkgo.label-filter" {
			continue
		}
		if !hasValue && i+1 < len(args) {
			value = args[i+1]
		}
		values = append(values, value)
	}
	return values
}

func wantsTSProjectBackend(args []string) bool {
	for _, filter := range labelFilters(args) {
		if positiveTSProjectBackend.MatchString(filter) {
			return true
		}
	}
	return false
}

// jsPlan decides what a run does about the installed js/semantics: a run that
// asks for the ts-project-backend label needs it, and is refused without it;
// any other run uses it when it is there, and otherwise says that the specs
// that need it will skip.
func jsPlan(args []string) (useJS bool, note string, err error) {
	_, statErr := os.Stat(jsInstalled)
	switch {
	case statErr == nil:
		return true, "", nil
	case wantsTSProjectBackend(args):
		return false, "", fmt.Errorf("the ts-project-backend label needs %s, which CI's job installs before its tests; run `mise run js-install` first", jsInstalled)
	default:
		return false, fmt.Sprintf("%s is not installed, so the ts-project-backend specs will skip; run `mise run js-install` to include them", jsInstalled), nil
	}
}

// copyJSInstalled dereferences links, so a node_modules that points into a
// directory the non-root user cannot read still arrives as readable files.
func copyJSInstalled(repoCopy string) error {
	target := filepath.Join(repoCopy, jsInstalled)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if out, err := exec.Command("cp", "-aL", jsInstalled, target).CombinedOutput(); err != nil {
		return fmt.Errorf("copying %s: %v: %s", jsInstalled, err, out)
	}
	return nil
}
