package evalgen

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// WriteTree writes a tree under dir; scripts are executable.
func WriteTree(dir string, tree map[string][]byte) error {
	for path, content := range tree {
		target := filepath.Join(dir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if strings.HasSuffix(path, ".sh") {
			mode = 0o755
		}
		if err := os.WriteFile(target, content, mode); err != nil {
			return err
		}
	}
	return nil
}

// Validate runs the native checks on a program: the stub compiles and vets, the visible tests pass on it, the
// hidden tests fail on it in their own package only, and the clean oracle passes everything. Nothing here
// reads Coach output.
func Validate(p Program) error {
	work, err := os.MkdirTemp("", "coach-eval-validate")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)

	steps := []struct {
		name  string
		files map[string][]byte
		run   [][]string
		fails []string
	}{
		{"stub", p.Tree, [][]string{{"build", "./..."}, {"vet", "./..."}, {"test", "./..."}}, nil},
		{"hidden tests on the stub", p.Tests, nil, []string{p.moduleName() + "/tests/feature"}},
		{"clean oracle", p.Solution, [][]string{{"vet", "./..."}, {"test", "./..."}}, nil},
	}
	for _, step := range steps {
		if err := WriteTree(work, step.files); err != nil {
			return err
		}
		for _, args := range step.run {
			if output, err := goIn(work, args...); err != nil {
				return fmt.Errorf("%s: go %s: %v\n%s", step.name, strings.Join(args, " "), err, output)
			}
		}
		if step.fails != nil {
			if err := requireOnlyFailures(work, step.fails); err != nil {
				return fmt.Errorf("%s: %w", step.name, err)
			}
		}
	}
	return nil
}

func (p Program) moduleName() string {
	first, _, _ := strings.Cut(strings.TrimPrefix(string(p.Tree["go.mod"]), "module "), "\n")
	return first
}

func requireOnlyFailures(dir string, want []string) error {
	output, err := goIn(dir, "test", "./...")
	if err == nil {
		return fmt.Errorf("go test passed, but the hidden tests must fail on the stub")
	}
	failed := []string{}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "FAIL" && strings.Contains(fields[1], "/") {
			failed = append(failed, fields[1])
		}
	}
	if strings.Join(failed, ",") != strings.Join(want, ",") {
		return fmt.Errorf("failing packages %v, want %v\n%s", failed, want, output)
	}
	return nil
}

func goIn(dir string, args ...string) (string, error) {
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOTOOLCHAIN=local", "GOPROXY=off", "GOWORK=off")
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	err := cmd.Run()
	return output.String(), err
}
