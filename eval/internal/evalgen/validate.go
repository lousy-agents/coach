package evalgen

import (
	"fmt"
	"os"
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
		{"stub", p.Tree, [][]string{{"build", "./..."}, {"vet", "./..."}, {"test", "./..."}}, nil},
		{"hidden tests on the stub", p.Tests, nil, []string{p.moduleName() + "/tests/feature"}},
		{"clean oracle", p.Solution, [][]string{{"vet", "./..."}, {"test", "./..."}}, nil},
	}
	for _, step := range steps {
		if err := step.run(work); err != nil {
			return err
		}
	}
	return nil
}

// validationStep overlays files on the work tree, then runs go commands that must pass or, when fails is set,
// a go test whose only failing packages must be exactly those.
type validationStep struct {
	name     string
	files    map[string][]byte
	commands [][]string
	fails    []string
}

func (step validationStep) run(work string) error {
	if err := WriteTree(work, step.files); err != nil {
		return err
	}
	for _, args := range step.commands {
		if output, err := goIn(work, args...); err != nil {
			return fmt.Errorf("%s: go %s: %v\n%s", step.name, strings.Join(args, " "), err, output)
		}
	}
	if step.fails == nil {
		return nil
	}
	if err := requireOnlyFailures(work, step.fails); err != nil {
		return fmt.Errorf("%s: %w", step.name, err)
	}
	return nil
}

func (p Program) moduleName() string {
	first, _, _ := strings.Cut(strings.TrimPrefix(string(p.Tree["go.mod"]), "module "), "\n")
	return first
}
