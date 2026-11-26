package evalgen

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Validate runs the native checks on a program: the stub compiles and vets, the visible tests pass on it, the
// hidden tests fail on it in their own package only, and the clean oracle passes everything. Nothing here
// reads Coach output.
func Validate(p Program) error {
	work, err := os.MkdirTemp("", "coach-eval-validate")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)

	steps := []validationStep{
		{id: StepNative, name: "stub", files: p.Tree, commands: [][]string{{"build", "./..."}, {"vet", "./..."}, {"test", "./..."}}},
		{id: StepNative, name: "hidden tests on the stub", files: p.Tests, fails: []string{p.moduleName() + "/tests/feature"}},
		{id: StepDeleteBaseline, name: "delete baseline", remove: p.entryFile(), target: "./tests/...", fails: []string{p.moduleName() + "/tests/feature"}},
		{id: StepNative, name: "clean oracle", files: p.Solution, commands: [][]string{{"vet", "./..."}, {"test", "./..."}}},
	}
	for _, step := range steps {
		if err := step.run(work); err != nil {
			return &StepError{Step: step.id, Err: err}
		}
	}
	return nil
}

// validationStep overlays files on the work tree, removes one file, then runs go commands that must pass or,
// when fails is set, a go test of target whose only failing packages must be exactly those.
type validationStep struct {
	id       string
	name     string
	files    map[string][]byte
	remove   string
	target   string
	commands [][]string
	fails    []string
}

func (step validationStep) run(work string) error {
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

// entryFile is the source file of the entry package, which the delete baseline removes.
func (p Program) entryFile() string {
	entry := p.Manifest.Template.APISurface[0].Entry
	dir := entry[:strings.LastIndex(entry, ".")]
	return dir + "/" + path.Base(dir) + ".go"
}

func (p Program) moduleName() string {
	first, _, _ := strings.Cut(strings.TrimPrefix(string(p.Tree["go.mod"]), "module "), "\n")
	return first
}
