package codesignalcli

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

var errNodeNotFound = errors.New("node executable not found on PATH")

// errNodeVersionProbeTimedOut signals that `node --version` did not
// complete within nodeVersionProbeTimeout; see checkNodeReadiness's doc for
// why this is classified separately from errNodeNotFound.
var errNodeVersionProbeTimedOut = errors.New("node --version timed out")

const (
	nodeVersionProbeTimeout   = 10 * time.Second
	maxNodeVersionProbeOutput = 4 << 10
)

// detectHostNodeMajor is the Node-detection seam: it looks up `node` on
// PATH and parses `node --version`'s major component. Tests may replace it
// to exercise node_missing/node_below_minimum without depending on the host
// environment's actual Node installation.
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

// checkNodeReadiness compares the discovered host Node major against
// MinimumSupportedNodeMajor and TestedNodeMajor: node_missing only when Node
// cannot be found on PATH at all (errNodeNotFound). Every other
// detectHostNodeMajor failure -- unparsable version output, a probe
// timeout, or a run failure -- means Node is actually present and was
// invoked but could not be confirmed to meet the floor, so it is reported
// as node_below_minimum with whatever diagnostic was observed rather than
// the misleading node_missing. A major below the floor is the same gap; a
// major at or above the floor but different from TestedNodeMajor is the
// node_untested warning, not a gap.
func checkNodeReadiness() ReadinessCheck {
	rawVersion, major, err := detectHostNodeMajor()
	if err != nil {
		if errors.Is(err, errNodeNotFound) {
			return ReadinessCheck{State: ReadinessFail, Code: GapNodeMissing}
		}
		foundVersion := rawVersion
		switch {
		case errors.Is(err, errNodeVersionProbeTimedOut):
			foundVersion = "timed out"
		case foundVersion == "":
			foundVersion = fmt.Sprintf("unparsable: %s", err)
		}
		return ReadinessCheck{State: ReadinessFail, Code: GapNodeBelowMinimum, FoundVersion: foundVersion}
	}
	if major < MinimumSupportedNodeMajor {
		return ReadinessCheck{State: ReadinessFail, Code: GapNodeBelowMinimum, FoundVersion: rawVersion}
	}
	if major != TestedNodeMajor {
		return ReadinessCheck{State: ReadinessPass, Code: WarnNodeUntested, Version: rawVersion}
	}
	return ReadinessCheck{State: ReadinessPass, Version: rawVersion}
}
