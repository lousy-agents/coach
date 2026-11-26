package evalharness

import "fmt"

func (c *hookChecker) prepare(source, work string, plan probePlan) error {
	if err := copyProgram(source, work); err != nil {
		return err
	}
	if plan.language == "typescript" {
		if err := provisionCompiler(c.tools.TSC, work); err != nil {
			return err
		}
	}
	for _, step := range [][]string{
		{"init", "-q"}, {"add", "-A"},
		{"-c", "user.name=coach-eval", "-c", "user.email=coach-eval@example.invalid", "commit", "-q", "-m", "initial"},
	} {
		if output, err := run(work, nil, "", "git", step...); err != nil {
			return fmt.Errorf("git %s in the copy of %s: %v: %s", step[0], source, err, output)
		}
	}
	return nil
}
