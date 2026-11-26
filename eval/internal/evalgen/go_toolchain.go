package evalgen

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

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
