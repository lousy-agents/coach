package codesignalcli

import (
	"encoding/json"
	"errors"
	"io/fs"
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
	// Yarn has no matrix row and is withheld before hazard/version checks
	// are consulted.
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
// repository-controlled configuration hazard from kind's Hazards column
// (SA-280-012), or "" if none. Only npm's hazard class is checked today;
// pnpm and Bun each need their own fixtures before their rows are wired in.
func detectPackageManagerHazard(root, kind string) string {
	if kind != packageManagerKindNPM {
		return ""
	}
	if detail := detectNpmrcHazard(root); detail != "" {
		return detail
	}
	return detectNpmLockfileHazard(root)
}

// detectNpmLockfileHazard reports a hazard detail when package-lock.json is
// absent or unreadable: the locked argv npm ci --ignore-scripts cannot run
// without it.
func detectNpmLockfileHazard(root string) string {
	if _, err := os.ReadFile(filepath.Join(root, "package-lock.json")); err != nil {
		return "package-lock.json is missing or could not be read"
	}
	return ""
}

// detectNpmrcHazard reports a hazard detail for a committed .npmrc that
// redirects the registry, re-enables lifecycle scripts, or overrides the
// script shell. A .npmrc that exists but cannot be read (permission denied,
// a directory, a dangling symlink) is a hazard in its own right, distinct
// from no .npmrc existing at all -- fail-closed rather than treating an
// unreadable hazard file as absent.
func detectNpmrcHazard(root string) string {
	path := filepath.Join(root, ".npmrc")
	data, err := os.ReadFile(path)
	if err != nil {
		if _, statErr := os.Lstat(path); errors.Is(statErr, fs.ErrNotExist) {
			return ""
		}
		return "committed .npmrc could not be read"
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
		key = strings.ToLower(strings.TrimSpace(key))
		value = trimNpmrcValueQuotes(strings.TrimSpace(value))
		switch {
		case key == "registry" || strings.HasSuffix(key, ":registry"):
			return "committed .npmrc redirects the package registry (" + key + "=" + value + ")"
		case key == "ignore-scripts":
			if value != "true" {
				return "committed .npmrc re-enables lifecycle scripts (ignore-scripts=" + value + ")"
			}
		case key == "script-shell":
			return "committed .npmrc overrides the lifecycle script shell (script-shell=" + value + ")"
		}
	}
	return ""
}

// trimNpmrcValueQuotes strips a single layer of matching double or single
// quotes from an ini-style value, per npmrc's quoting rules.
func trimNpmrcValueQuotes(value string) string {
	if len(value) >= 2 {
		first, last := value[0], value[len(value)-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			return value[1 : len(value)-1]
		}
	}
	return value
}
