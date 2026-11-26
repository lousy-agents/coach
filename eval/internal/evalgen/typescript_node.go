package evalgen

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// requireNodeMajor refuses a node older than MinimumNodeMajor, because it cannot run the generated .ts tests.
func requireNodeMajor(node string) error {
	output, err := exec.Command(node, "--version").Output()
	version := strings.TrimSpace(string(output))
	major, parsed := strings.CutPrefix(version, "v")
	if err != nil || !parsed {
		return fmt.Errorf("%s did not report a version: node %d or later is needed to run TypeScript tests", node, MinimumNodeMajor)
	}
	number, _, _ := strings.Cut(major, ".")
	if value, convErr := strconv.Atoi(number); convErr != nil || value < MinimumNodeMajor {
		return fmt.Errorf("node %s is older than the node %d that runs TypeScript tests without a flag", version, MinimumNodeMajor)
	}
	return nil
}
