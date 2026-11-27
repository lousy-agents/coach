package validationtasks

import (
	"fmt"
	"strings"
)

// jobRunCommands returns the run: value of each step of the job, as YAML decodes it.
func jobRunCommands(workflow, job string) []string {
	var runs []string
	for _, step := range jobSteps(workflowJob(workflow, job)) {
		if run, ok := step["run"].(string); ok {
			runs = append(runs, strings.TrimSpace(run))
		}
	}
	return runs
}

// setupSteps returns each uses: step of the job with its with: values, as text: the steps that make the machine
// (checkout, mise, the Go cache), and not the checks that run on it.
func setupSteps(job map[string]any) []string {
	var steps []string
	for _, step := range jobSteps(job) {
		if uses, ok := step["uses"].(string); ok {
			steps = append(steps, uses+" "+stringify(step["with"]))
		}
	}
	return steps
}

// stepEnv returns the env of the step of the job whose run is run.
func stepEnv(job map[string]any, run string) string {
	for _, step := range jobSteps(job) {
		if strings.TrimSpace(stringify(step["run"])) == run {
			return stringify(step["env"])
		}
	}
	return "no step runs " + run
}

func jobSteps(job map[string]any) []map[string]any {
	var steps []map[string]any
	raw, _ := job["steps"].([]any)
	for _, entry := range raw {
		step, _ := entry.(map[string]any)
		steps = append(steps, step)
	}
	return steps
}

func stringify(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}
