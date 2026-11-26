package evalgen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// validationStep overlays files on the work tree, removes one file, then runs go commands that must pass or,
// when fails is set, a go test of target whose only failing packages must be exactly those.
type validationStep struct {
	drop     []string
	id       string
	name     string
	files    map[string][]byte
	remove   string
	target   string
	commands [][]string
	fails    []string
}

// followOnDrops are removed before the follow-on tests run: the follow-on changes the entry's contract, so the
// first feature's hidden tests stop describing it. The follow-on tests assert both fields in their place.
var followOnDrops = []string{"tests/feature"}

func (step validationStep) run(work string) error {
	for _, path := range step.drop {
		if err := os.RemoveAll(filepath.Join(work, filepath.FromSlash(path))); err != nil {
			return err
		}
	}
	if err := WriteTree(work, step.files); err != nil {
		return err
	}
	if step.remove != "" {
		if err := os.Remove(filepath.Join(work, filepath.FromSlash(step.remove))); err != nil {
			return err
		}
	}
	for _, args := range step.commands {
		if output, err := goIn(work, args...); err != nil {
			return fmt.Errorf("%s: go %s: %v\n%s", step.name, strings.Join(args, " "), err, output)
		}
	}
	if step.fails == nil {
		return nil
	}
	if err := requireOnlyFailures(work, step.target, step.fails); err != nil {
		return fmt.Errorf("%s: %w", step.name, err)
	}
	return nil
}
