package codesignalcli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/lousy-agents/coach/internal/codesignalcli/subprocess"
)

const (
	miseProbeTimeout   = 10 * time.Second
	maxMiseProbeOutput = 4 << 10
)

// runMiseProbe runs one read-only `mise` subcommand -- never `mise install`
// or any other mutating subcommand. ok is false unless the probe ran fully
// confined, so a probe that cannot be confined does not run at all. A
// non-zero exit is reported in exitErr rather than as a failed probe.
func runMiseProbe(ctx context.Context, args ...string) (out string, exitErr error, ok bool) {
	if _, lookErr := exec.LookPath("mise"); lookErr != nil {
		return "", nil, false
	}
	// The working directory is private per probe rather than a fixed shared
	// path: mise resolves configuration by walking up from it, so a
	// predictable path under TMPDIR would let another local user plant
	// configuration the probe then loads.
	probeDir, tempErr := os.MkdirTemp("", "coach-mise-probe-")
	if tempErr != nil {
		return "", nil, false
	}
	defer func() { _ = os.RemoveAll(probeDir) }()

	data, exitErr, probeErr := subprocess.ProbeAt(ctx, miseProbeTimeout, maxMiseProbeOutput, probeDir, miseProbeEnv(), "mise", args...)
	if probeErr != nil {
		return "", nil, false
	}
	return strings.TrimSpace(string(data)), exitErr, true
}

func miseProbeEnv() []string {
	env := []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
	}
	for _, key := range []string{"MISE_DATA_DIR", "MISE_CONFIG_DIR"} {
		if value := os.Getenv(key); value != "" {
			env = append(env, key+"="+value)
		}
	}
	return env
}

// readMiseProjectConfigFile reads a project's mise.toml for hazard scanning.
// A missing or unreadable file is not itself a hazard signal here --
// evaluateMiseProjectOrigin already reports that distinctly (unconfigured or
// unreadable); this just has nothing to scan.
func readMiseProjectConfigFile(dir string) (string, bool) {
	data, err := os.ReadFile(filepath.Join(dir, "mise.toml"))
	if err != nil {
		return "", false
	}
	return string(data), true
}

func miseProjectConfigReadable(dir string) bool {
	_, ok := readMiseProjectConfigFile(dir)
	return ok
}

// miseProjectConfigExists distinguishes a scope that configures nothing from
// one Coach cannot verify: Lstat, not Stat, so a dangling symlink counts as
// present-but-unreadable rather than absent.
func miseProjectConfigExists(dir string) bool {
	_, err := os.Lstat(filepath.Join(dir, "mise.toml"))
	return err == nil
}
