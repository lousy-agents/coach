package evalgrade

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

//go:embed typescript_nonce_reporter.mjs
var reporterSource []byte

const (
	runTimeout = 10 * time.Minute
)

// RunTypeScriptTests runs the given test files, each in its own process, under a reporter that gets the nonce on a
// descriptor it reads and closes before any test runs. Only the reporter, in node's main process, holds the nonce and
// record descriptors; the test processes node starts inherit neither.
func RunTypeScriptTests(node, dir string, files []string, nonce string) (TypeScriptRun, error) {
	return RunTypeScriptTestsWithin(node, dir, files, nonce, runTimeout)
}

// RunTypeScriptTestsWithin is RunTypeScriptTests with the time after which node is killed.
func RunTypeScriptTestsWithin(node, dir string, files []string, nonce string, timeout time.Duration) (TypeScriptRun, error) {
	if err := checkRun(node, files, nonce); err != nil {
		return TypeScriptRun{}, err
	}
	reporter, cleanup, err := writeReporter()
	if err != nil {
		return TypeScriptRun{}, err
	}
	defer cleanup()
	return runWithChannels(node, dir, reporter, files, nonce, timeout)
}

func checkRun(node string, files []string, nonce string) error {
	if nonce == "" || nonce != strings.TrimSpace(nonce) || strings.ContainsAny(nonce, "\r\n") {
		return errors.New("the nonce must be one non-empty line with no space at either end")
	}
	if len(files) == 0 {
		return errors.New("no test file to run")
	}
	return nil
}

func writeReporter() (path string, cleanup func(), err error) {
	dir, err := os.MkdirTemp("", "evalgrade-reporter-")
	if err != nil {
		return "", nil, err
	}
	path = filepath.Join(dir, "reporter.mjs")
	if err := os.WriteFile(path, reporterSource, 0o600); err != nil {
		_ = os.RemoveAll(dir)
		return "", nil, err
	}
	return path, func() { _ = os.RemoveAll(dir) }, nil
}

func runWithChannels(node, dir, reporter string, files []string, nonce string, timeout time.Duration) (TypeScriptRun, error) {
	nonceRead, nonceWrite, err := os.Pipe()
	if err != nil {
		return TypeScriptRun{}, err
	}
	defer nonceRead.Close()
	if _, err := nonceWrite.WriteString(nonce + "\n"); err != nil {
		return TypeScriptRun{}, err
	}
	_ = nonceWrite.Close()
	recordRead, recordWrite, err := os.Pipe()
	if err != nil {
		return TypeScriptRun{}, err
	}
	defer recordRead.Close()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	args := append([]string{"--test", "--test-isolation=process", "--test-reporter=" + reporter}, files...)
	var output bytes.Buffer
	cmd := nodeCommand(ctx, node, dir, args, []*os.File{nonceRead, recordWrite}, &output)
	run, err := supervised{cmd: cmd, ctx: ctx, reads: recordRead, writes: recordWrite}.run()
	if err != nil {
		return TypeScriptRun{}, fmt.Errorf("node did not start: %v", err)
	}
	run.Output = output.String()
	return run, nil
}
