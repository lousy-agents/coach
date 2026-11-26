package antipatternfixtures

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Vetter builds and vets the Go module in dir.
type Vetter func(dir string) error

// GoVet runs go vet over the module in dir offline, with the local
// toolchain and no workspace, so a fixture that would not build or vet in
// #871's harness is caught here.
func GoVet(dir string) error {
	cmd := exec.Command("go", "vet", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOTOOLCHAIN=local", "GOPROXY=off", "GOWORK=off")
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(output.String()))
	}
	return nil
}

func vetVariant(files map[string]string, vet Vetter) error {
	dir, err := os.MkdirTemp("", "antipatternfixtures-vet-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	if err := writeFiles(dir, files, ""); err != nil {
		return err
	}
	return vet(dir)
}

func writeFiles(dir string, files map[string]string, suffix string) error {
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name)+suffix)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}
