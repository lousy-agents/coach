package tstoolchain

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/lousy-agents/coach/internal/codesignalcli/subprocess"
)

var (
	ErrHostNodeNotFound        = errors.New("node executable not found on PATH")
	ErrHostNodeMajorDisallowed = errors.New("host node major is outside the analysis runtime set")
)

const (
	hostNodeVersionProbeTimeout   = 10 * time.Second
	maxHostNodeVersionProbeOutput = 4 << 10
)

func AnalysisNodeMajorAllowed(major int) bool {
	return major == 24 || major == 26
}

func hostNodeProbeEnv() []string {
	return []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
	}
}

func mapHostNodeProbeError(path, probe string, exitErr, probeErr error) error {
	switch {
	case errors.Is(probeErr, subprocess.ErrProbeTimedOut):
		return fmt.Errorf("%s %s timed out", path, probe)
	case probeErr != nil:
		return probeErr
	case exitErr != nil:
		return fmt.Errorf("running %s %s: %w", path, probe, exitErr)
	default:
		return nil
	}
}

// ResolveHostNode resolves the exact host `node` executable PrepareTSRuntime
// spawns the analyzer with: process.execPath from a probe of the LookPath
// result (never a version-manager shim) and its raw `node --version` output.
// This is a separate probe from CheckNode/detectHostNodeMajor
// (project_readiness.go): readiness only needs a major version for
// --check-project, while runtime preparation needs the resolved absolute
// path itself to spawn against, and the two are independent probes by
// design -- see ResolveCompilerForRuntime's doc comment for the analogous
// compiler-side distinction between "readiness pass" and "runtime
// resolvable."
var ResolveHostNode = func(ctx context.Context) (execPath, rawVersion string, err error) {
	path, lookErr := exec.LookPath("node")
	if lookErr != nil {
		return "", "", ErrHostNodeNotFound
	}

	data, exitErr, probeErr := subprocess.ProbeAt(ctx, hostNodeVersionProbeTimeout, maxHostNodeVersionProbeOutput, "", hostNodeProbeEnv(), path, "--version")
	if err := mapHostNodeProbeError(path, "--version", exitErr, probeErr); err != nil {
		return "", "", err
	}

	rawVersion = strings.TrimSpace(string(data))
	major, parseErr := ParseNodeMajor(rawVersion)
	if parseErr != nil {
		return "", "", parseErr
	}
	if !AnalysisNodeMajorAllowed(major) {
		return "", "", ErrHostNodeMajorDisallowed
	}

	execData, execExitErr, execProbeErr := subprocess.ProbeAt(ctx, hostNodeVersionProbeTimeout, maxHostNodeVersionProbeOutput, "", hostNodeProbeEnv(), path, "-p", "process.execPath")
	if err := mapHostNodeProbeError(path, "process.execPath probe", execExitErr, execProbeErr); err != nil {
		return "", "", err
	}

	execPath = strings.TrimSpace(string(execData))
	if !filepath.IsAbs(execPath) {
		return "", "", fmt.Errorf("host node process.execPath is not absolute: %q", execPath)
	}
	return execPath, rawVersion, nil
}
