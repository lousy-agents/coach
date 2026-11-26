package evalharness

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
)

// runIn runs a program in dir with extra environment entries and standard input, and returns its combined output.
func runIn(dir string, env []string, stdin, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdin = strings.NewReader(stdin)
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	err := cmd.Run()
	return output.String(), err
}
