package evalharness

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// PinnedGremlinsVersion is the Gremlins the eval module's tool directive requires; CheckVersion fails if
// eval/go.mod moves without it, so every host judges mutants with the same tool.
const PinnedGremlinsVersion = "v0.6.0"

const gremlinsModule = "github.com/go-gremlins/gremlins"

// ErrGremlinsUnavailable marks a pinned Gremlins that cannot be built from the module cache.
var ErrGremlinsUnavailable = errors.New("evalharness: pinned gremlins unavailable offline")

// offlineGoEnv keeps `go tool` from downloading a module or a toolchain: it is applied after any caller
// environment, so a caller cannot reopen lookup.
var offlineGoEnv = []string{"GOPROXY=off", "GOTOOLCHAIN=local"}

// Gremlins runs a pinned Gremlins binary.
type Gremlins struct {
	Binary  string
	Version string
}

// ResolveGremlins builds the eval module's pinned Gremlins from the module cache alone and checks its version.
// env adds to the inherited environment. Pinning the tool is a development-time download; running the gate
// never fetches.
func ResolveGremlins(evalModuleDir string, env []string) (Gremlins, error) {
	var stdout, stderr bytes.Buffer
	command := exec.Command("go", "tool", "-n", "gremlins")
	command.Dir = evalModuleDir
	command.Env = append(append(os.Environ(), env...), offlineGoEnv...)
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		return Gremlins{}, fmt.Errorf("%w: %v: %s; download it once with `go mod download` in the eval module",
			ErrGremlinsUnavailable, err, strings.TrimSpace(stderr.String()))
	}
	gremlins := Gremlins{Binary: strings.TrimSpace(stdout.String()), Version: PinnedGremlinsVersion}
	return gremlins, gremlins.CheckVersion()
}

// CheckVersion refuses a binary built from any Gremlins module version other than the pinned one.
func (g Gremlins) CheckVersion() error {
	output, err := exec.Command("go", "version", "-m", g.Binary).Output()
	if err != nil {
		return fmt.Errorf("read gremlins build info: %w", err)
	}
	version, found := gremlinsModuleVersion(string(output))
	if !found {
		return fmt.Errorf("gremlins binary %s carries no %s module version; the pinned version is %s", g.Binary, gremlinsModule, g.Version)
	}
	if version != g.Version {
		return fmt.Errorf("gremlins is %s, but the pinned version is %s", version, g.Version)
	}
	return nil
}

// gremlinsModuleVersion reads the Gremlins module's version from `go version -m` output.
func gremlinsModuleVersion(buildInfo string) (string, bool) {
	for _, line := range strings.Split(buildInfo, "\n") {
		if fields := strings.Fields(line); len(fields) >= 3 && fields[0] == "mod" && fields[1] == gremlinsModule {
			return fields[2], true
		}
	}
	return "", false
}
