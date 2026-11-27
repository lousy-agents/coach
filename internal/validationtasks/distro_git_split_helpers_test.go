package validationtasks

import (
	"slices"
	"strings"
)

const distroCommandWord = "./cmd/acceptance-ci-env"

// distroCommands returns the run: value of each step of the job that runs acceptance-ci-env, in step order.
func distroCommands(workflow, job string) []string {
	var commands []string
	for _, run := range jobRunCommands(workflow, job) {
		if strings.Contains(run, distroCommandWord) {
			commands = append(commands, run)
		}
	}
	return commands
}

// commandsNaming returns the commands whose package arguments are exactly packages.
func commandsNaming(commands []string, packages ...string) []string {
	var named []string
	for _, command := range commands {
		if slices.Equal(packageArguments(command), packages) {
			named = append(named, command)
		}
	}
	return named
}

// distroSetupSteps returns every step of the job that does not run acceptance-ci-env, as text: the steps that make
// the machine and the container, and not the tests that run on it.
func distroSetupSteps(job map[string]any) []string {
	var steps []string
	for _, step := range jobSteps(job) {
		if !strings.Contains(stringify(step["run"]), distroCommandWord) {
			steps = append(steps, stringify(step))
		}
	}
	return steps
}
