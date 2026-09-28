package codesignalcli

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// resolveHostNode resolves the exact host `node` executable PrepareTSRuntime
// spawns the analyzer with: process.execPath from a probe of the LookPath
// result (never a version-manager shim) and its raw `node --version` output.
// This is a separate probe from checkNodeReadiness/detectHostNodeMajor
// (project_readiness.go): readiness only needs a major version for
// --check-project, while runtime preparation needs the resolved absolute
// path itself to spawn against, and the two are independent probes by
// design -- see resolveCompilerForRuntime's doc comment for the analogous
// compiler-side distinction between "readiness pass" and "runtime
// resolvable."
var resolveHostNode = func(ctx context.Context) (execPath, rawVersion string, err error) {
	path, lookErr := exec.LookPath("node")
	if lookErr != nil {
		return "", "", errHostNodeNotFound
	}

	data, exitErr, probeErr := runBoundedSubprocessProbeAt(ctx, hostNodeVersionProbeTimeout, maxHostNodeVersionProbeOutput, "", hostNodeProbeEnv(), path, "--version")
	if err := mapHostNodeProbeError(path, "--version", exitErr, probeErr); err != nil {
		return "", "", err
	}

	rawVersion = strings.TrimSpace(string(data))
	major, parseErr := parseNodeMajor(rawVersion)
	if parseErr != nil {
		return "", "", parseErr
	}
	if !analysisNodeMajorAllowed(major) {
		return "", "", errHostNodeMajorDisallowed
	}

	execData, execExitErr, execProbeErr := runBoundedSubprocessProbeAt(ctx, hostNodeVersionProbeTimeout, maxHostNodeVersionProbeOutput, "", hostNodeProbeEnv(), path, "-p", "process.execPath")
	if err := mapHostNodeProbeError(path, "process.execPath probe", execExitErr, execProbeErr); err != nil {
		return "", "", err
	}

	execPath = strings.TrimSpace(string(execData))
	if !filepath.IsAbs(execPath) {
		return "", "", fmt.Errorf("host node process.execPath is not absolute: %q", execPath)
	}
	return execPath, rawVersion, nil
}
