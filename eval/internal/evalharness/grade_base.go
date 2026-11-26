package evalharness

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/lousy-agents/coach/eval/internal/evaldetect"
	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

// GradeBase runs the graded detectors on an unmutated tree: neither may report anything. A finding rejects the
// program at the detector's step; a detector that cannot run is an error that is not a rejection.
func GradeBase(root string) error {
	policy, err := evaldetect.LoadPolicy(filepath.Join(root, ".lousy-agents", "coach.json"))
	if err != nil {
		return err
	}
	layerFindings, err := evaldetect.GoLayerViolations(root, policy)
	if err != nil {
		return err
	}
	if len(layerFindings) > 0 {
		return &evalgen.StepError{Step: evalgen.StepLayerGraph, Err: reported(layerFindings)}
	}
	return gradeComplexity(root)
}

func gradeComplexity(root string) error {
	module, err := findEvalModule()
	if err != nil {
		return err
	}
	gocognit, err := evaldetect.ResolveGocognit(module, nil)
	if err != nil {
		return err
	}
	findings, err := gocognit.Findings(root)
	if err != nil {
		return err
	}
	if len(findings) > 0 {
		return &evalgen.StepError{Step: evalgen.StepGocognit, Err: reported(findings)}
	}
	return nil
}

func reported(findings []evaldetect.Finding) error {
	lines := make([]string, 0, len(findings))
	for _, finding := range findings {
		lines = append(lines, fmt.Sprintf("%s %s:%d %s", finding.RuleID, finding.Path, finding.Line, finding.Subject))
	}
	return fmt.Errorf("the graded detectors report:\n%s", strings.Join(lines, "\n"))
}
