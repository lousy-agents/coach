package codesignalcli

import (
	"bytes"
	"context"

	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// readCommittedBunfig reads root's bunfig.toml. present is false both when
// there is genuinely no file (detail "": nothing to be hazardous) and when
// one exists but cannot be read (detail names the hazard) -- the fail-closed
// half AC-SET-12 requires, since an unreadable config is not a safe one.
func readCommittedBunfig(root string) (data []byte, detail string, present bool) {
	path := filepath.Join(root, "bunfig.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		if _, statErr := os.Lstat(path); errors.Is(statErr, fs.ErrNotExist) {
			return nil, "", false
		}
		return nil, "committed bunfig.toml could not be read", false
	}
	return bytes.TrimPrefix(data, []byte("\xEF\xBB\xBF")), "", true
}

// detectPackageManagerHazard reports a non-empty detail describing a
// repository-controlled configuration hazard from kind's Hazards column
// (SA-280-012), or "" if none. The lockfile-readability precondition itself
// is kind-agnostic; see requireReadableLockfile.
func detectPackageManagerHazard(root, kind string) string {
	switch kind {
	case packageManagerKindNPM:
		return detectNpmrcHazard(root)
	case packageManagerKindPNPM:
		return detectNpmrcRegistryHazard(root)
	case packageManagerKindBun:

		if detail := detectNpmrcRegistryHazard(root); detail != "" {
			return detail
		}
		return detectBunfigHazard(root)
	default:
		return ""
	}
}
func classifyProbedPackageManagerVersion(detection packageManagerDetection) ReadinessCheck {
	version, probed := probePackageManagerVersion(context.Background(), detection.kind)
	if !probed || !isExactVersion(version) {
		return ReadinessCheck{State: ReadinessFail, Code: GapPackageManagerVersionUnverifiable, Kind: detection.kind, PinnedVersion: detection.pin}
	}
	if !packageManagerVersionSupported(detection.kind, version) {
		return ReadinessCheck{State: ReadinessFail, Code: GapPackageManagerVersionUnsupported, Kind: detection.kind, FoundVersion: version, PinnedVersion: detection.pin}
	}
	return ReadinessCheck{State: ReadinessPass, Kind: detection.kind, Version: version, PinnedVersion: detection.pin}
}

// trimConfigValueQuotes strips a single layer of matching double or single
// quotes from a config value, per .npmrc's ini quoting rules -- also
// sufficient for a TOML basic/literal string's outer quotes in
// detectBunfigHazard's narrow scan.
func trimConfigValueQuotes(value string) string {
	if len(value) >= 2 {
		first, last := value[0], value[len(value)-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			return value[1 : len(value)-1]
		}
	}
	return value
}
