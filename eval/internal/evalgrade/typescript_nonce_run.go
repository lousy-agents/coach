package evalgrade

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
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
// descriptor it reads and closes before any test runs. Tests inherit the record descriptor but never the nonce.
func RunTypeScriptTests(node, dir string, files []string, nonce string) (TypeScriptRun, error) {
	if err := checkRun(node, files, nonce); err != nil {
		return TypeScriptRun{}, err
	}
	reporter, cleanup, err := writeReporter()
	if err != nil {
		return TypeScriptRun{}, err
	}
	defer cleanup()
	return runWithChannels(node, dir, reporter, files, nonce)
}

func checkRun(node string, files []string, nonce string) error {
	if nonce == "" || strings.ContainsAny(nonce, "\r\n") {
		return errors.New("the nonce must be one non-empty line")
	}
	if len(files) == 0 {
		return errors.New("no test file to run")
	}
	if _, err := os.Stat(node); err != nil {
		return fmt.Errorf("node is not usable at %s: %v", node, err)
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

func runWithChannels(node, dir, reporter string, files []string, nonce string) (TypeScriptRun, error) {
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

	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	defer cancel()
	args := append([]string{"--test", "--test-isolation=process", "--test-reporter=" + reporter}, files...)
	cmd := exec.CommandContext(ctx, node, args...)
	cmd.Dir = dir
	cmd.ExtraFiles = []*os.File{nonceRead, recordWrite}
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Start(); err != nil {
		_ = recordWrite.Close()
		return TypeScriptRun{}, fmt.Errorf("node did not start: %v", err)
	}
	_ = recordWrite.Close()
	run := readRecords(recordRead)
	_ = cmd.Wait()
	run.Output = output.String()
	return run, nil
}
