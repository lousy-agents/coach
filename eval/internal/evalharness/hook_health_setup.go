package evalharness

import (
	"os"
	"path/filepath"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

func newHookChecker(coach string, programs []healthProgram) (*hookChecker, error) {
	checker := &hookChecker{plans: map[string]probePlan{}}
	wantsTypeScript := false
	for _, program := range programs {
		plan, err := planProbe(program.Path)
		if err != nil {
			return nil, err
		}
		checker.plans[program.Path] = plan
		wantsTypeScript = wantsTypeScript || plan.language == "typescript"
	}
	if wantsTypeScript {
		workingDir, _ := os.Getwd()
		tools, err := evalgen.ResolveTypeScriptTools(workingDir)
		if err != nil {
			return nil, err
		}
		checker.tools = tools
	}
	dir, err := os.MkdirTemp("", "coach-eval-hook-health-bin")
	if err != nil {
		return nil, err
	}
	absolute, err := filepath.Abs(coach)
	if err != nil {
		return nil, err
	}
	if err := os.Symlink(absolute, filepath.Join(dir, "coach")); err != nil {
		return nil, err
	}
	checker.coachDir = dir
	return checker, nil
}
