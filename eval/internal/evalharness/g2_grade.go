package evalharness

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/lousy-agents/coach/eval/internal/evaldetect"
	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

// gradeG2 reports what the graded detectors find on an oracle: a forbidden edge, or a reach for the grading channel.
func gradeG2(program evalgen.Program, tree oracleTree) error {
	directory, err := os.MkdirTemp("", "coach-eval-g2")
	if err != nil {
		return err
	}
	defer os.RemoveAll(directory)
	if err := evalgen.WriteTree(directory, tree.files); err != nil {
		return err
	}
	policy, err := evaldetect.LoadPolicy(filepath.Join(directory, ".lousy-agents", "coach.json"))
	if err != nil {
		return err
	}
	layers, err := evaldetect.GoLayerViolations(directory, policy)
	if err != nil {
		return err
	}
	channel, err := evaldetect.GradingChannelAccess(directory)
	if err != nil {
		return err
	}
	absent, err := evaldetect.APISurfaceAbsences(directory, surfaceSymbols(program))
	if err != nil {
		return err
	}
	missing, err := evaldetect.RequiredEdgeAbsences(directory, surfaceEdges(program))
	if err != nil {
		return err
	}
	if findings := slices.Concat(layers, channel, absent, missing); len(findings) > 0 {
		return &evalgen.StepError{Step: evalgen.StepG2, Err: fmt.Errorf("%s oracle: %w", tree.name, reported(findings))}
	}
	return nil
}
