package codesignalcli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
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

	data, exitErr, probeErr := runBoundedSubprocessProbeAt(ctx, miseProbeTimeout, maxMiseProbeOutput, probeDir, miseProbeEnv(), "mise", args...)
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

// detectGlobalMiseTypescriptVersion reports the globally configured
// TypeScript version. A non-zero exit means the key is unset, not that the
// probe failed.
var detectGlobalMiseTypescriptVersion = func(ctx context.Context) (version string, found bool) {
	out, _, ok := runMiseProbe(ctx, "config", "get", "tools.npm:typescript", "-g")
	if !ok || out == "" {
		return "", false
	}
	return out, true
}

// locateMiseTypescriptInstall reports where mise installed the requested
// version. `mise where` reports the tool's install root, not the npm package
// directory: the `typescript` package lives at
// <install-root>/node_modules/typescript.
var locateMiseTypescriptInstall = func(ctx context.Context, version string) (string, bool) {
	toolRoot, exitErr, ok := runMiseProbe(ctx, "where", "npm:typescript@"+version)
	if !ok || exitErr != nil || toolRoot == "" {
		return "", false
	}
	pkgDir := filepath.Join(toolRoot, "node_modules", "typescript")
	if _, statErr := os.Stat(filepath.Join(pkgDir, "package.json")); statErr != nil {
		return "", false
	}
	return pkgDir, true
}
