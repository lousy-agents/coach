package evaldetect

import (
	"fmt"
	"os/exec"
	"strings"
)

// CheckVersion refuses a binary built from any gocognit module version other
// than the pinned one, so every host grades with the same counting rules.
func (g Gocognit) CheckVersion() error {
	output, err := exec.Command("go", "version", "-m", g.Binary).Output()
	if err != nil {
		return fmt.Errorf("read gocognit build info: %w", err)
	}
	version, ok := moduleVersion(string(output), gocognitModule)
	if ok && version != g.Version {
		return fmt.Errorf("gocognit is %s, but the pinned version is %s", version, g.Version)
	}
	if ok {
		return nil
	}
	return fmt.Errorf("gocognit binary %s carries no %s module version; the pinned version is %s", g.Binary, gocognitModule, g.Version)
}

// moduleVersion reads a module's version from `go version -m` output.
func moduleVersion(buildInfo, module string) (string, bool) {
	for _, line := range strings.Split(buildInfo, "\n") {
		if fields := strings.Fields(line); len(fields) >= 3 && fields[0] == "mod" && fields[1] == module {
			return fields[2], true
		}
	}
	return "", false
}
