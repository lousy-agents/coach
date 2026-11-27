package validationtasks

import (
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const coachPackage = "github.com/lousy-agents/coach/cmd/coach"

// flagValues returns the value of every `-name=value` argument of command.
func flagValues(command, name string) []string {
	var values []string
	for _, word := range strings.Fields(command) {
		if value, found := strings.CutPrefix(word, "-"+name+"="); found {
			values = append(values, value)
		}
	}
	return values
}

// packageArguments returns the arguments of command that name packages, as go test reads them: those that start
// with ./.
func packageArguments(command string) []string {
	var packages []string
	for _, word := range strings.Fields(command) {
		if strings.HasPrefix(word, "./") {
			packages = append(packages, word)
		}
	}
	return packages
}

// goList returns the import paths the go tool names for the arguments, from the repository root.
func goList(arguments ...string) []string {
	GinkgoHelper()
	command := exec.Command("go", append([]string{"list"}, arguments...)...)
	command.Dir = filepath.Join("..", "..")
	output, err := command.Output()
	Expect(err).NotTo(HaveOccurred(), "go list %v", arguments)
	return strings.Fields(string(output))
}

func policyCommand(miseToml string) string {
	GinkgoHelper()
	commands := taskRunCommands(miseToml, "test-policy")
	Expect(commands).To(HaveLen(1), "test-policy is one go test command")
	return commands[0]
}

func restCommands(miseToml string) []string {
	GinkgoHelper()
	commands := taskRunCommands(miseToml, "test-rest")
	Expect(commands).To(HaveLen(2), "test-rest is the other packages, then cmd/coach's remaining specs")
	return commands
}
