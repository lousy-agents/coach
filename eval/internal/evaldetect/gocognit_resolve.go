package evaldetect

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// PinnedGocognitVersion is the gocognit the eval module's tool directive
// requires; CheckVersion fails if eval/go.mod moves without it.
const PinnedGocognitVersion = "v1.3.0"

// offlineGoEnv keeps `go tool` from downloading a module or a toolchain, so
// a run never fetches silently. It is applied after any caller environment,
// so a caller cannot reopen lookup or request a toolchain the host lacks.
var offlineGoEnv = []string{"GOPROXY=off", "GOTOOLCHAIN=local"}

// ResolveGocognit builds the eval module's pinned gocognit from the module
// cache alone and checks its version. env adds to the inherited environment.
func ResolveGocognit(evalModuleDir string, env []string) (Gocognit, error) {
	return resolveGocognit(evalModuleDir, env, PinnedGocognitVersion)
}

func resolveGocognit(evalModuleDir string, env []string, pinned string) (Gocognit, error) {
	var stdout, stderr bytes.Buffer
	command := exec.Command("go", "tool", "-n", "gocognit")
	command.Dir = evalModuleDir
	command.Env = append(append(os.Environ(), env...), offlineGoEnv...)
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		return Gocognit{}, fmt.Errorf("%w: %v: %s; download it once with `go mod download` in the eval module",
			ErrGocognitUnavailable, err, strings.TrimSpace(stderr.String()))
	}
	gocognit := Gocognit{Binary: strings.TrimSpace(stdout.String()), Version: pinned}
	return gocognit, gocognit.CheckVersion()
}
