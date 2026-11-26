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

// withBaseFindings returns the program with what coach reports on its unmutated base in the manifest. It never
// rejects a program: a count is recorded, and a coach that is absent or fails is recorded as null with the
// reason (design §6, "coach_base_fp").
func withBaseFindings(coach coachBinary, program evalgen.Program) evalgen.Program {
	validation := make(map[string]any, len(program.Manifest.Validation)+2)
	for key, value := range program.Manifest.Validation {
		validation[key] = value
	}
	for key, value := range baseFindingFields(coach, program.Tree) {
		validation[key] = value
	}
	program.Manifest.Validation = validation
	return program
}

func baseFindingFields(coach coachBinary, tree map[string][]byte) map[string]any {
	if !coach.found() {
		return map[string]any{baseFindingsKey: nil, baseFindingsReason: noCoachBinary}
	}
	count, err := coachSignalCount(coach.path, tree)
	if err != nil {
		return map[string]any{baseFindingsKey: nil, baseFindingsReason: "coach failed: " + err.Error()}
	}
	return map[string]any{baseFindingsKey: count}
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
