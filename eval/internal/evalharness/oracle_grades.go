package evalharness

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/lousy-agents/coach/eval/internal/evaldetect"
	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

// OracleGrades counts the tests each oracle reported as passed under native G1.
type OracleGrades struct {
	FirstOracleTests    int
	FollowOnOracleTests int
}

// GradeOracles runs G2 on each oracle of a Go program, then native G1: the tests the program lists and the hidden
// tests must each report pass. A finding or a test that did not pass rejects the program at its own step. Native G1 is
// go test's own report, not the nonce channel (#791), so nothing it records may be read as that G1.
func GradeOracles(program evalgen.Program) (OracleGrades, error) {
	trees := oracleTrees(program)
	for _, tree := range trees {
		if err := gradeG2(program, tree); err != nil {
			return OracleGrades{}, err
		}
	}
	var counts []int
	for _, tree := range trees {
		count, err := gradeNativeG1(program, tree)
		if err != nil {
			return OracleGrades{}, err
		}
		counts = append(counts, count)
	}
	return OracleGrades{FirstOracleTests: counts[0], FollowOnOracleTests: counts[1]}, nil
}

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
