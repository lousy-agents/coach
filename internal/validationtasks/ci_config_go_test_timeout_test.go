package validationtasks

import (
	"regexp"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v4"
)

var tomlString = regexp.MustCompile(`(?m)^\s*(?:run\s*=\s*\[?\s*)?("(?:[^"\\]|\\.)*")\s*,?\s*\]?\s*$`)

// taskRunCommands returns the string entries of a mise task's `run`, decoded
// as TOML basic strings, so a trailing comment inside an entry stays part of
// the entry.
func taskRunCommands(toml, task string) []string {
	var commands []string
	for _, match := range tomlString.FindAllStringSubmatch(taskSteps(toml, task), -1) {
		if command, err := strconv.Unquote(match[1]); err == nil {
			commands = append(commands, command)
		}
	}
	return commands
}

func taskGoTestTimeout(toml, task string) (int, bool) {
	for _, command := range taskRunCommands(toml, task) {
		if minutes, ok := goTestTimeoutMinutes(command); ok {
			return minutes, true
		}
	}
	return 0, false
}

func workflowJob(workflow, name string) map[string]any {
	var document struct {
		Jobs map[string]map[string]any `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(workflow), &document); err != nil {
		return nil
	}
	return document.Jobs[name]
}

// jobGoTestTimeout reads the run: value of each step of the job, as YAML
// decodes it, so a comment line or a trailing comment is not part of the value.
// It names a timeout only when every step that runs go test or acceptance-ci-env
// names one, and returns the smallest, so a job of several runs is held to its
// weakest.
func jobGoTestTimeout(workflow, job string) (int, bool) {
	smallest, found := 0, false
	for _, run := range jobRunCommands(workflow, job) {
		if !runsGoTest(run) {
			continue
		}
		minutes, ok := goTestTimeoutMinutes(run)
		if !ok {
			return 0, false
		}
		if !found || minutes < smallest {
			smallest, found = minutes, true
		}
	}
	return smallest, found
}

func runsGoTest(run string) bool {
	return strings.Contains(run, "go test") || strings.Contains(run, "./cmd/acceptance-ci-env")
}

func jobTimeoutMinutes(workflow, job string) (int, bool) {
	minutes, ok := workflowJob(workflow, job)["timeout-minutes"].(int)
	return minutes, ok
}
