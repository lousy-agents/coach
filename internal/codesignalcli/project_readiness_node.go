package codesignalcli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

var errNodeNotFound = errors.New("node executable not found on PATH")
var errNodeVersionProbeTimedOut = errors.New("node --version timed out")

const (
	nodeVersionProbeTimeout   = 10 * time.Second
	maxNodeVersionProbeOutput = 4 << 10
)

// Test seam: replaced in tests to exercise node_missing/node_unsupported/
// node_unverifiable without depending on the host's actual Node install.
var detectHostNodeMajor = func() (rawVersion string, major int, err error) {
	if _, lookErr := exec.LookPath("node"); lookErr != nil {
		return "", 0, errNodeNotFound
	}

	data, exitErr, probeErr := runBoundedSubprocessProbe(context.Background(), nodeVersionProbeTimeout, maxNodeVersionProbeOutput, "node", "--version")
	switch {
	case errors.Is(probeErr, errBoundedProbeTimedOut):
		return "", 0, errNodeVersionProbeTimedOut
	case probeErr != nil:
		return "", 0, probeErr
	case exitErr != nil:
		return "", 0, fmt.Errorf("running node --version: %w", exitErr)
	}

	rawVersion = strings.TrimSpace(string(data))
	major, err = parseNodeMajor(rawVersion)
	return rawVersion, major, err
}

func parseNodeMajor(rawVersion string) (int, error) {
	trimmed := strings.TrimPrefix(rawVersion, "v")
	majorPart, _, _ := strings.Cut(trimmed, ".")
	major, err := strconv.Atoi(majorPart)
	if err != nil {
		return 0, fmt.Errorf("unparsable node version %q", rawVersion)
	}
	return major, nil
}

const (
	readinessNodeCheckKind   = "node"
	readinessNodeCheckOrigin = "path"
)

func checkNodeReadiness() ReadinessCheck {
	rawVersion, major, err := detectHostNodeMajor()
	if err != nil {
		if errors.Is(err, errNodeNotFound) {
			return ReadinessCheck{State: ReadinessFail, Code: GapNodeMissing, Kind: readinessNodeCheckKind, Origin: readinessNodeCheckOrigin}
		}
		return ReadinessCheck{State: ReadinessFail, Code: GapNodeUnverifiable, Kind: readinessNodeCheckKind, Origin: readinessNodeCheckOrigin, Detail: nodeUnverifiableDetail(err, rawVersion)}
	}
	if !nodeMajorSupported(major) {
		return ReadinessCheck{State: ReadinessFail, Code: GapNodeUnsupported, Kind: readinessNodeCheckKind, Origin: readinessNodeCheckOrigin, Version: rawVersion}
	}
	return ReadinessCheck{State: ReadinessPass, Kind: readinessNodeCheckKind, Origin: readinessNodeCheckOrigin, Version: rawVersion}
}

// A separate, smaller bound: maxNodeVersionProbeOutput (4 KiB) sizes the
// subprocess read, not the rendered diagnostic, and %q escaping can inflate
// it further.
const maxNodeUnverifiableDetailRawVersion = 200

func nodeUnverifiableDetail(err error, rawVersion string) string {
	var pathErr *fs.PathError
	switch {
	case errors.Is(err, errNodeVersionProbeTimedOut):
		return "node --version timed out"
	case rawVersion != "":
		return fmt.Sprintf("node --version printed an unparsable version: %q", truncateNodeUnverifiableRawVersion(rawVersion))
	case errors.As(err, &pathErr):
		return fmt.Sprintf("node --version failed to start: %s", truncateNodeUnverifiableRawVersion(pathErr.Err.Error()))
	default:
		return fmt.Sprintf("node --version failed: %s", err)
	}
}

func truncateNodeUnverifiableRawVersion(rawVersion string) string {
	if len(rawVersion) <= maxNodeUnverifiableDetailRawVersion {
		return rawVersion
	}
	return rawVersion[:maxNodeUnverifiableDetailRawVersion] + "...(truncated)"
}

// Kind, Origin, and Detail are deliberately never mirrored onto checks.node.
func nodeCompatibilityMirror(runtimeCheck ReadinessCheck) ReadinessCheck {
	return ReadinessCheck{State: runtimeCheck.State, Code: runtimeCheck.Code, Version: runtimeCheck.Version}
}
