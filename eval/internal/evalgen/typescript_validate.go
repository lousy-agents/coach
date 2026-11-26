package evalgen

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
)

// ValidateTypeScript runs the native checks on a program: the stub type-checks, the visible tests pass on it, the
// hidden tests fail on it and nowhere else, and the clean oracle passes everything. Nothing here reads Coach output.
func ValidateTypeScript(p Program, tools TypeScriptTools) error {
	work, err := os.MkdirTemp("", "coach-eval-validate-ts")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)

	visible := testFiles(p.Tree)
	hidden := testFiles(p.Tests)
	steps := []struct {
		name  string
		files map[string][]byte
		check func() error
	}{
		{"stub", p.Tree, func() error { return tools.passes(work, "stub", visible) }},
		{"hidden tests on the stub", p.Tests, func() error { return tools.failsOnlyTheHidden(work, visible, hidden) }},
		{"clean oracle", p.Solution, func() error { return tools.passes(work, "clean oracle", append(visible, hidden...)) }},
	}
	for _, step := range steps {
		if err := WriteTree(work, step.files); err != nil {
			return err
		}
		if err := step.check(); err != nil {
			return err
		}
	}
	return nil
}

func testFiles(tree map[string][]byte) []string {
	files := []string{}
	for path := range tree {
		if strings.HasSuffix(path, ".test.ts") {
			files = append(files, path)
		}
	}
	sort.Strings(files)
	return files
}

// passes type-checks the tree and requires every test file to pass.
func (t TypeScriptTools) passes(dir, step string, tests []string) error {
	if output, err := run(dir, t.TSC, "-p", "."); err != nil {
		return fmt.Errorf("%s: tsc: %v\n%s", step, err, output)
	}
	if output, err := t.nodeTest(dir, tests); err != nil {
		return fmt.Errorf("%s: node --test: %v\n%s", step, err, output)
	}
	return nil
}

// failsOnlyTheHidden requires the stub to still type-check, every visible test to still pass, and every hidden test
// file to fail.
func (t TypeScriptTools) failsOnlyTheHidden(dir string, visible, hidden []string) error {
	if err := t.passes(dir, "hidden tests on the stub", visible); err != nil {
		return err
	}
	for _, file := range hidden {
		if output, err := t.nodeTest(dir, []string{file}); err == nil {
			return fmt.Errorf("hidden tests on the stub: %s passed, but must fail on the stub\n%s", file, output)
		}
	}
	return nil
}

func (t TypeScriptTools) nodeTest(dir string, files []string) (string, error) {
	return run(dir, t.Node, append([]string{"--test", "--test-isolation=process"}, files...)...)
}

func run(dir, program string, args ...string) (string, error) {
	cmd := exec.Command(program, args...)
	cmd.Dir = dir
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	err := cmd.Run()
	return output.String(), err
}
