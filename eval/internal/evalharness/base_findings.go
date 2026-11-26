package evalharness

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

const (
	coachBaseTimeout   = 5 * time.Minute
	noCoachBinary      = "no coach binary"
	baseFindingsKey    = "coach_base_fp"
	baseFindingsReason = "coach_base_fp_reason"
)

// recordBaseFindings puts what coach reports on the unmutated base into the manifest. It never rejects a
// program: a count is recorded, and a coach that is absent or fails is recorded as null with the reason
// (design §6, "coach_base_fp").
func recordBaseFindings(coach coachBinary, program *evalgen.Program) {
	if program.Manifest.Validation == nil {
		program.Manifest.Validation = map[string]any{}
	}
	validation := program.Manifest.Validation
	if !coach.found() {
		validation[baseFindingsKey], validation[baseFindingsReason] = nil, noCoachBinary
		return
	}
	count, err := coachSignalCount(coach.path, program.Tree)
	if err != nil {
		validation[baseFindingsKey], validation[baseFindingsReason] = nil, "coach failed: "+err.Error()
		return
	}
	validation[baseFindingsKey] = count
}

func coachSignalCount(coach string, tree map[string][]byte) (int, error) {
	directory, err := os.MkdirTemp("", "coach-eval-base")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(directory)
	if err := evalgen.WriteTree(directory, tree); err != nil {
		return 0, err
	}
	if err := commitTree(directory); err != nil {
		return 0, err
	}
	return runCoachBaseline(coach, directory)
}

func commitTree(directory string) error {
	commands := [][]string{
		{"init", "-q"},
		{"add", "-A"},
		{"-c", "user.email=eval@example.invalid", "-c", "user.name=coach-eval", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "base"},
	}
	for _, args := range commands {
		command := exec.Command("git", args...)
		command.Dir = directory
		if output, err := command.CombinedOutput(); err != nil {
			return fmt.Errorf("git %s: %v: %s", args[0], err, output)
		}
	}
	return nil
}

func runCoachBaseline(coach, directory string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), coachBaseTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, coach, "codesignal", "--baseline", "--format=json")
	command.Dir = directory
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		return 0, fmt.Errorf("%v: %s", err, bytes.TrimSpace(stderr.Bytes()))
	}
	var report struct {
		Signals []json.RawMessage `json:"signals"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		return 0, fmt.Errorf("the report is not JSON: %v", err)
	}
	return len(report.Signals), nil
}
