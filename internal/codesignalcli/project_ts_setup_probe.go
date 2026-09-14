package codesignalcli

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	packageManagerProbeTimeout   = 10 * time.Second
	maxPackageManagerProbeOutput = 4 << 10
)

// runPackageManagerProbe runs one read-only `--version` style subcommand of a
// package manager -- never an install or any other mutating subcommand. ok is
// false unless the probe ran fully confined, so a probe that cannot be
// confined does not run at all. A non-zero exit is reported in exitErr rather
// than as a failed probe.
func runPackageManagerProbe(ctx context.Context, executable string, args ...string) (out string, exitErr error, ok bool) {
	if _, lookErr := exec.LookPath(executable); lookErr != nil {
		return "", nil, false
	}
	// The working directory is private per probe rather than the repository
	// under analysis: npm, pnpm, and bun all resolve configuration by walking
	// up from the working directory, so probing inside the repository would
	// let a committed config file change what the probe reports.
	probeDir, tempErr := os.MkdirTemp("", "coach-package-manager-probe-")
	if tempErr != nil {
		return "", nil, false
	}
	defer func() { _ = os.RemoveAll(probeDir) }()

	data, exitErr, probeErr := runBoundedSubprocessProbeAt(ctx, packageManagerProbeTimeout, maxPackageManagerProbeOutput, probeDir, packageManagerProbeEnv(), executable, args...)
	if probeErr != nil {
		return "", nil, false
	}
	return strings.TrimSpace(string(data)), exitErr, true
}

// packageManagerProbeEnv is PATH and HOME alone: PATH so the manager resolves
// its own runtime (npm needs node), HOME because npm and pnpm both refuse to
// start without one. Nothing else is forwarded, so no ambient registry,
// cache, or script-shell variable can steer what the probe reports.
func packageManagerProbeEnv() []string {
	return []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
	}
}

// probePackageManagerVersion reports the version of the manager binary that
// would actually run for kind. ok is false whenever the probe could not be
// confined, the manager is absent from PATH, it exited non-zero, or it printed
// nothing parseable -- every one of those is "undetectable", never
// "unsupported".
func probePackageManagerVersion(ctx context.Context, kind string) (string, bool) {
	out, exitErr, ok := runPackageManagerProbe(ctx, kind, "--version")
	if !ok || exitErr != nil || out == "" {
		return "", false
	}
	return packageManagerVersionToken(out)
}

// packageManagerVersionToken extracts the leading version token from a
// manager's `--version` output. npm, pnpm, and bun each print a bare version
// on one line; taking the first token of the first line tolerates a manager
// that appends a build or platform suffix without treating that suffix as
// part of the version.
func packageManagerVersionToken(versionOutput string) (string, bool) {
	line, _, _ := strings.Cut(strings.TrimSpace(versionOutput), "\n")
	token, _, _ := strings.Cut(strings.TrimSpace(line), " ")
	if token == "" {
		return "", false
	}
	return token, true
}
