package gitrepo

import (
	"errors"
	"strings"
	"testing"
	"time"
)

const (
	generousStdoutBytes = 1 << 10
	generousStderrBytes = 1 << 10
	runnerTimeout       = 10 * time.Second
)

func TestBoundedRunnerReturnsWhatGitPrinted(t *testing.T) {
	run := BoundedRunner(generousStdoutBytes, generousStderrBytes, runnerTimeout)

	output, err := run(t.TempDir(), "--version")

	if err != nil {
		t.Fatalf("run --version: %v", err)
	}
	if !strings.HasPrefix(string(output), "git version") {
		t.Errorf("output = %q, want git's version line", output)
	}
}

func TestBoundedRunnerHoldsEveryRunToItsLimits(t *testing.T) {
	const oneByte = 1
	run := BoundedRunner(oneByte, generousStderrBytes, runnerTimeout)

	_, err := run(t.TempDir(), "--version")

	var bound *BoundError
	if !errors.As(err, &bound) || bound.Kind != BoundStdout {
		t.Errorf("err = %v, want a stdout-bound failure: a runner that ignored its limits would return git's whole line", err)
	}
}
