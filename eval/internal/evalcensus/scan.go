package evalcensus

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// CoachRun is one Coach baseline scan of a checkout: its JSON report, or the
// exit status and the evidence of its failure.
type CoachRun struct {
	Report  []byte
	Exit    int
	Failure string
}

// RunCoach runs Coach's baseline scan in the checkout at dir. coach is an
// absolute path, because the scan runs from inside the checkout.
func RunCoach(repo Repository, coach, dir string) CoachRun {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(coach, coachArgs(repo)...)
	cmd.Dir = dir
	cmd.Env = isolatedEnv()
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		exit := exitCode(err)
		return CoachRun{Exit: exit, Failure: fmt.Sprintf("coach exited %d: %s", exit, strings.TrimSpace(stderr.String()))}
	}
	return CoachRun{Report: stdout.Bytes()}
}

// scanRepository turns a Coach scan of the checkout, or its failure, into
// rule statuses.
func scanRepository(corpus Corpus, repo Repository, coach, dir string) RepositoryResult {
	result := RepositoryResult{ID: repo.ID, SHA: repo.SHA, Sample: &RepositorySample{Seed: sampleSeed(repo)}}
	run := RunCoach(repo, coach, dir)
	if run.Failure == "" {
		result.Statuses = StatusesFromReport(corpus, repo, run.Report)
		return result
	}
	result.CoachExit = run.Exit
	result.Statuses = StatusesFromFailure(corpus, repo, run.Failure)
	return result
}

func coachArgs(repo Repository) []string {
	args := []string{"codesignal", "--baseline", "--format", "json", "--no-interactive", "--fail-on-incomplete-coverage"}
	if repo.ProjectConfig != "" {
		args = append(args, "--project-config", repo.ProjectConfig)
	}
	return args
}

// exitCode returns a process's exit status, or -1 when it never ran.
func exitCode(err error) int {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}
