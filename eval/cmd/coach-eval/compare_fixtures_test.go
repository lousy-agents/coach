package main_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/gomega"
)

type record = map[string]any

// completeRecord is a run.json that is valid under schema version 1 and comparable to any other
// completeRecord, so each spec changes exactly one input.
func completeRecord(runID string) record {
	return record{
		"schema_version":  "1",
		"run_id":          runID,
		"state":           "complete",
		"coach_sha":       "0123456789abcdef0123456789abcdef01234567",
		"worktree_dirty":  false,
		"protocol_hash":   "sha256:protocol",
		"solver":          record{"name": "claude-code", "version": "2.1.0", "model": "claude-test-model"},
		"isolation":       "docker_sandbox",
		"image":           "sha256:image",
		"runtime":         "runc",
		"architecture":    "amd64",
		"grader_versions": record{"typescript": "7.0.2", "gocognit": "v1.2.0"},
		"caps":            record{"trials": 360, "tokens": 1000000, "turns": 50, "timeout_seconds": 600, "run_hours": 10},
		"prompt_hash":     "sha256:prompt",
		"oracle_hash":     "sha256:oracle",
		"parameters":      record{"k": 3, "instances_per_cell": 2, "seed": 7},
		"usage":           record{"tokens": 1200, "turns": 14, "hours": 0.5},
		"results":         record{},
	}
}

// withPath returns a copy of r with the leaf at a dot-separated path, for example "caps.trials",
// set to value. r is not modified, so a spec's base record stays intact.
func withPath(r record, path string, value any) record {
	return withKeys(r, strings.Split(path, "."), value)
}

func withKeys(r record, keys []string, value any) record {
	copied := make(record, len(r)+1)
	maps.Copy(copied, r)
	if len(keys) == 1 {
		copied[keys[0]] = value
		return copied
	}
	copied[keys[0]] = withKeys(r[keys[0]].(record), keys[1:], value)
	return copied
}

func writeRunRecord(stateRoot, runID string, r record) {
	dir := filepath.Join(stateRoot, "coach-eval", runID)
	Expect(os.MkdirAll(dir, 0o755)).To(Succeed())
	data, err := json.MarshalIndent(r, "", "  ")
	Expect(err).NotTo(HaveOccurred())
	Expect(os.WriteFile(filepath.Join(dir, "run.json"), data, 0o600)).To(Succeed())
}

type compareOutcome struct {
	exitCode int
	stdout   string
	stderr   string
}

// runCompare runs the built coach-eval binary, so exit codes are the contract under test.
func runCompare(stateRoot string, runIDs ...string) compareOutcome {
	return runCommand(stateRoot, "", coachEvalBinary, append([]string{"compare"}, runIDs...)...)
}

// runCompareThroughMise runs the eval-compare mise task from the repository root.
func runCompareThroughMise(stateRoot string, runIDs ...string) compareOutcome {
	return runCommand(stateRoot, filepath.Join("..", "..", ".."), "mise", append([]string{"run", "eval-compare", "--"}, runIDs...)...)
}

func runCommand(stateRoot, dir, name string, args ...string) compareOutcome {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "XDG_STATE_HOME="+stateRoot)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		Expect(err).NotTo(HaveOccurred())
	}
	code := 0
	if exitErr != nil {
		code = exitErr.ExitCode()
	}
	return compareOutcome{exitCode: code, stdout: stdout.String(), stderr: stderr.String()}
}
