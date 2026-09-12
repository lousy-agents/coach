package codesignalcli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// checkPackageManager detects and classifies the repository's package
// manager against the frozen adapter support matrix (SA-280-012), reading
// worktree state at dir's repository root the same way resolveCompiler
// does. No install is ever executed here -- an ambiguous, unverifiable, or
// hazardous finding fails closed with a distinct gap code (SA-280-015)
// instead.
func checkPackageManager(dir string) ReadinessCheck {
	root := compilerWorktreeRoot(dir)

	detection, ok := detectPackageManager(root)
	if !ok {
		return ReadinessCheck{State: ReadinessNotChecked}
	}
	if detection.ambiguous {
		return ReadinessCheck{State: ReadinessFail, Code: GapPackageManagerAmbiguous}
	}
	// Yarn is deliberately excluded from the matrix (owner decision): it is
	// withheld unconditionally, without consulting hazards or version, since
	// there is no supported row for it to satisfy.
	if detection.kind == packageManagerKindYarn {
		return ReadinessCheck{
			State:  ReadinessFail,
			Code:   GapPackageManagerVersionUnsupported,
			Kind:   detection.kind,
			Detail: "Yarn has no supported package-manager adapter row (SA-280-012); use npm, pnpm, or Bun instead.",
		}
	}
	if detail := detectPackageManagerHazard(root, detection.kind); detail != "" {
		return ReadinessCheck{State: ReadinessFail, Code: GapPackageManagerConfigUnverifiable, Kind: detection.kind, Detail: detail}
	}
	if detection.version == "" || !isExactVersion(detection.version) {
		return ReadinessCheck{State: ReadinessFail, Code: GapPackageManagerVersionUnverifiable, Kind: detection.kind}
	}
	if !packageManagerVersionSupported(detection.kind, detection.version) {
		return ReadinessCheck{State: ReadinessFail, Code: GapPackageManagerVersionUnsupported, Kind: detection.kind, FoundVersion: detection.version}
	}
	return ReadinessCheck{State: ReadinessPass, Kind: detection.kind, Version: detection.version}
}

type packageManagerDetection struct {
	kind      string
	version   string // pinned exact version from package.json's packageManager field, "" if absent
	ambiguous bool
}

// detectPackageManager identifies the repository's package manager from its
// recognized metadata (SA-280-012): a package.json "packageManager" pin
// takes precedence over a bare lockfile, since the pin also carries the
// exact version a lockfile alone cannot. ok is false only when no recognized
// metadata exists at all, distinct from detection.ambiguous (recognized
// metadata that disagrees with itself).
func detectPackageManager(root string) (packageManagerDetection, bool) {
	fieldKind, fieldVersion, fieldOK := readPackageManagerField(root)
	lockKind, lockAmbiguous, lockOK := detectPackageManagerLockfile(root)

	if lockAmbiguous {
		return packageManagerDetection{ambiguous: true}, true
	}
	switch {
	case fieldOK && lockOK && fieldKind != lockKind:
		return packageManagerDetection{ambiguous: true}, true
	case fieldOK:
		return packageManagerDetection{kind: fieldKind, version: fieldVersion}, true
	case lockOK:
		return packageManagerDetection{kind: lockKind}, true
	default:
		return packageManagerDetection{}, false
	}
}

// readPackageManagerField reads package.json's Corepack-style
// "packageManager": "<name>@<version>" field. An unrecognized manager name
// is treated the same as an absent field, falling back to lockfile
// detection instead.
func readPackageManagerField(root string) (kind, version string, ok bool) {
	data, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		return "", "", false
	}
	var manifest struct {
		PackageManager string `json:"packageManager"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil || manifest.PackageManager == "" {
		return "", "", false
	}
	name, rest, found := strings.Cut(manifest.PackageManager, "@")
	if !found || rest == "" {
		return "", "", false
	}
	switch name {
	case packageManagerKindNPM, packageManagerKindPNPM, packageManagerKindBun, packageManagerKindYarn:
		return name, rest, true
	default:
		return "", "", false
	}
}

// detectPackageManagerLockfile reports the manager kind implied by a
// recognized lockfile basename present at root. More than one distinct
// kind's lockfile committed simultaneously is reported as ambiguous rather
// than picking one arbitrarily.
func detectPackageManagerLockfile(root string) (kind string, ambiguous bool, ok bool) {
	found := map[string]bool{}
	for basename, k := range packageManagerLockfileBasenames {
		if fileExists(filepath.Join(root, basename)) {
			found[k] = true
		}
	}
	switch len(found) {
	case 0:
		return "", false, false
	case 1:
		for k := range found {
			return k, false, true
		}
	}
	return "", true, true
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// detectPackageManagerHazard reports a non-empty detail describing a
// repository-controlled configuration hazard matching kind's Hazards column
// (SA-280-012), or "" if none is found. Only npm's ".npmrc registry
// redirect" hazard class has dedicated fixture coverage today (issue #327
// Task 1); the pnpm/Bun branches exist so their own hazard classes can be
// added under Task 7 without restructuring this seam.
func detectPackageManagerHazard(root, kind string) string {
	switch kind {
	case packageManagerKindNPM, packageManagerKindPNPM:
		return detectNpmrcHazard(root)
	case packageManagerKindBun:
		return detectBunHazard(root)
	default:
		return ""
	}
}

func detectNpmrcHazard(root string) string {
	data, err := os.ReadFile(filepath.Join(root, ".npmrc"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		switch key {
		case "registry":
			return "committed .npmrc redirects the package registry (registry=" + value + ")"
		case "ignore-scripts":
			if value == "false" {
				return "committed .npmrc re-enables lifecycle scripts (ignore-scripts=false)"
			}
		case "script-shell":
			return "committed .npmrc overrides the lifecycle script shell (script-shell=" + value + ")"
		}
	}
	return ""
}

func detectBunHazard(root string) string {
	if hasTrustedDependencies(root) {
		return "package.json declares trustedDependencies without a verified script-suppression proof"
	}
	data, err := os.ReadFile(filepath.Join(root, "bunfig.toml"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "registry") || strings.Contains(line, "install.registry") {
			return "committed bunfig.toml redirects the package registry or an install hook"
		}
	}
	return ""
}

func hasTrustedDependencies(root string) bool {
	data, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		return false
	}
	var manifest struct {
		TrustedDependencies []string `json:"trustedDependencies"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return false
	}
	return len(manifest.TrustedDependencies) > 0
}
