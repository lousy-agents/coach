package evalcensus

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// scanRepository runs Coach's baseline scan in the checked-out repository
// and turns its report, or its failure, into rule statuses. coach is an
// absolute path, because the scan runs from inside the checkout.
func scanRepository(corpus Corpus, repo Repository, coach, dir string) RepositoryResult {
	result := RepositoryResult{ID: repo.ID, SHA: repo.SHA}
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(coach, coachArgs(repo)...)
	cmd.Dir = dir
	cmd.Env = isolatedEnv()
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if err == nil {
		result.Statuses = StatusesFromReport(corpus, repo, stdout.Bytes())
		return result
	}
	result.CoachExit = exitCode(err)
	evidence := fmt.Sprintf("coach exited %d: %s", result.CoachExit, strings.TrimSpace(stderr.String()))
	result.Statuses = StatusesFromFailure(corpus, repo, evidence)
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
