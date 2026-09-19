package codesignalcli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// checkPackageManager detects and classifies the package manager of every
// selected root against the frozen adapter support matrix (SA-280-012),
// reading worktree state at the same per-root context resolveCompiler
// resolves a compiler against. No install is ever executed here -- an
// ambiguous, unverifiable, or hazardous finding fails closed with a distinct
// gap code (SA-280-015) instead.
//
// The version that classifies the row is probed from the manager binary that
// would actually run (probePackageManagerVersion), never taken from
// package.json's packageManager pin. The frozen adapter rows invoke the bare
// executable name and Coach never installs or switches to a pinned release,
// so a pin describes an intention while the probe describes what will run;
// classifying by the pin would clear a row for a binary that is not there.
// The pin is still recorded, and BuildSetupPreview discloses it whenever it
// disagrees with the probe.
func checkPackageManager(dir string, roots []string) ReadinessCheck {
	contexts := packageManagerContexts(compilerWorktreeRoot(dir), roots)

	detection, ok := detectPackageManagerAcrossContexts(contexts)
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
			State:         ReadinessFail,
			Code:          GapPackageManagerVersionUnsupported,
			Kind:          detection.kind,
			PinnedVersion: detection.pin,
			Detail:        "Yarn has no supported package-manager adapter row (SA-280-012); use npm, pnpm, or Bun instead.",
		}
	}
	// Every context is a directory the frozen adapter row could be asked to
	// install in, so a hazard or a missing lockfile at any one of them is
	// disqualifying -- not only at the first that happened to resolve a kind.
	for _, packageDir := range contexts {
		if detail := detectPackageManagerHazard(packageDir, detection.kind); detail != "" {
			return packageManagerConfigUnverifiable(detection, detail)
		}
		if detail := requireReadableLockfile(packageDir, detection.kind); detail != "" {
			return packageManagerConfigUnverifiable(detection, detail)
		}
	}
	return classifyProbedPackageManagerVersion(detection)
}

// packageManagerContexts resolves the directories checkPackageManager reads
// manager metadata from: each selected root's nearest package.json, which is
// exactly the context resolveCompiler resolves that root's compiler against
// (resolveProjectRoot). A root with no manifest at or above it contributes no
// context; when no root contributes one, the worktree root stands in, so a
// repository whose only metadata is a top-level lockfile beside no
// package.json is still classified rather than silently unchecked.
func packageManagerContexts(worktreeRoot string, roots []string) []string {
	if len(roots) == 0 {
		roots = []string{"."}
	}
	contexts := make([]string, 0, len(roots))
	for _, root := range roots {
		if manifestDir, ok := nearestPackageJSONDir(selectedRootAbs(worktreeRoot, root), worktreeRoot); ok {
			contexts = append(contexts, manifestDir)
		}
	}
	if len(contexts) == 0 {
		return []string{worktreeRoot}
	}
	return dedupeStrings(contexts)
}

// detectPackageManagerAcrossContexts reduces the selected roots' package
// contexts to one detection. Contexts naming different managers are reported
// as ambiguous rather than resolved to whichever root was listed first: the
// frozen rows install one manager in one working directory, so there is no
// honest way to serve two. A context with no recognized metadata contributes
// nothing and is not itself a disagreement. ok is false only when no context
// recognized anything at all.
func detectPackageManagerAcrossContexts(contexts []string) (packageManagerDetection, bool) {
	var resolved packageManagerDetection
	found := false
	for _, packageDir := range contexts {
		detection, ok := detectPackageManager(packageDir)
		if !ok {
			continue
		}
		if detection.ambiguous {
			return packageManagerDetection{ambiguous: true}, true
		}
		if !found {
			resolved, found = detection, true
			continue
		}
		if detection.kind != resolved.kind {
			return packageManagerDetection{ambiguous: true}, true
		}
		if resolved.pin == "" {
			resolved.pin = detection.pin
		}
	}
	return resolved, found
}

func packageManagerConfigUnverifiable(detection packageManagerDetection, detail string) ReadinessCheck {
	return ReadinessCheck{State: ReadinessFail, Code: GapPackageManagerConfigUnverifiable, Kind: detection.kind, PinnedVersion: detection.pin, Detail: detail}
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

type packageManagerDetection struct {
	kind      string
	pin       string // package.json's packageManager version, recorded for disclosure only
	ambiguous bool
}

// detectPackageManager identifies the repository's package manager from its
// recognized metadata (SA-280-012): a package.json "packageManager" pin and a
// bare lockfile are equally good evidence of which manager a repository uses,
// and they are cross-checked against each other rather than ranked. ok is
// false only when no recognized metadata exists at all, distinct from
// detection.ambiguous (recognized metadata that disagrees with itself).
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
		return packageManagerDetection{kind: fieldKind, pin: fieldVersion}, true
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
// (SA-280-012), or "" if none. The lockfile-readability precondition itself
// is kind-agnostic; see requireReadableLockfile.
func detectPackageManagerHazard(root, kind string) string {
	switch kind {
	case packageManagerKindNPM:
		return detectNpmrcHazard(root)
	case packageManagerKindPNPM:
		return detectNpmrcRegistryHazard(root)
	case packageManagerKindBun:
		// Bun resolves its install registry from a committed .npmrc the same
		// way npm/pnpm do, on top of bunfig.toml rather than instead of it
		// (verified empirically against real Bun 1.3.11: `bun install
		// --ignore-scripts` still fails with ConnectionRefused against a
		// .npmrc-redirected host with no bunfig.toml present at all) -- so
		// both must be checked, not just bunfig.toml.
		if detail := detectNpmrcRegistryHazard(root); detail != "" {
			return detail
		}
		return detectBunfigHazard(root)
	default:
		return ""
	}
}

// requireReadableLockfile reports a hazard detail unless at least one of
// kind's recognized lockfile basenames (SA-280-012) exists and can be read
// at root: none of the matrix-recognized managers' locked install argv can
// run without one. A basename that is missing and one that exists but
// cannot be read are treated identically -- both fail closed -- unlike
// detectNpmrcHazard, where absence of the (optional) file is itself safe.
func requireReadableLockfile(root, kind string) string {
	for basename, k := range packageManagerLockfileBasenames {
		if k != kind {
			continue
		}
		if _, err := os.ReadFile(filepath.Join(root, basename)); err == nil {
			return ""
		}
	}
	return "no readable " + kind + " lockfile was found (SA-280-012)"
}

// scanNpmrcLines reads root's committed .npmrc (the ini-style config file
// both npm and pnpm honor -- verified empirically that `pnpm config get`
// resolves a committed .npmrc's keys the same way npm does) and calls handle
// with each non-comment, non-blank line's lowercased key and unquoted value,
// stopping at the first non-empty detail handle returns. Absence of .npmrc
// is safe ("" with handle never called); a .npmrc that exists but cannot be
// read (permission denied, a directory, a dangling symlink) is a hazard in
// its own right, distinct from no .npmrc existing at all -- fail-closed
// rather than treating an unreadable hazard file as absent.
func scanNpmrcLines(root string, handle func(key, value string) string) string {
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
		value = trimConfigValueQuotes(strings.TrimSpace(value))
		if detail := handle(key, value); detail != "" {
			return detail
		}
	}
	return ""
}

// isNpmrcRegistryKey reports whether key is .npmrc's global "registry"
// setting or a scoped "@scope:registry" override. Shared between npm's
// detectNpmrcHazard and detectNpmrcRegistryHazard (pnpm and Bun), since both
// read the same .npmrc format npm does for their own registry resolution.
func isNpmrcRegistryKey(key string) bool {
	return key == "registry" || strings.HasSuffix(key, ":registry")
}

// detectNpmrcHazard reports a hazard detail for a committed .npmrc that
// redirects the registry, re-enables lifecycle scripts, or overrides the
// script shell -- npm's full Hazards column (SA-280-012).
func detectNpmrcHazard(root string) string {
	return scanNpmrcLines(root, func(key, value string) string {
		switch {
		case isNpmrcRegistryKey(key):
			return "committed .npmrc redirects the package registry (" + key + "=" + value + ")"
		case key == "ignore-scripts":
			if !strings.EqualFold(value, "true") {
				return "committed .npmrc re-enables lifecycle scripts (ignore-scripts=" + value + ")"
			}
		case key == "script-shell":
			return "committed .npmrc overrides the lifecycle script shell (script-shell=" + value + ")"
		}
		return ""
	})
}

// detectNpmrcRegistryHazard reports a hazard detail for a committed .npmrc
// that redirects the package registry, shared between pnpm and Bun's
// detection (verified empirically against pnpm 10.33.0 and Bun 1.3.11: `pnpm
// config get registry` and `bun install --ignore-scripts` both honor a
// committed .npmrc's registry key the same way npm does -- Bun does so even
// with no bunfig.toml present at all). pnpm's own enable-pre-post-scripts
// setting and its onlyBuiltDependencies build-script allowlist were each
// verified empirically, against real pnpm 10.33.0, NOT to re-enable a
// dependency's postinstall script under this package's frozen `pnpm install
// --frozen-lockfile --ignore-scripts --ignore-pnpmfile` argv -- neither is
// checked here, since refusing on a setting that is not an actual bypass
// would be inventing a hazard rather than fail-closed.
func detectNpmrcRegistryHazard(root string) string {
	return scanNpmrcLines(root, func(key, value string) string {
		if isNpmrcRegistryKey(key) {
			return "committed .npmrc redirects the package registry (" + key + "=" + value + ")"
		}
		return ""
	})
}

// detectBunfigHazard reports a hazard detail for a committed bunfig.toml
// that carries any unverified [install]-namespaced configuration: a registry
// redirect (globally under [install] or for a specific scope either under
// [install.scopes] or via [install]'s own "scopes" table), or an
// [install.cache] "dir" redirect -- verified empirically against real Bun
// 1.3.11 that `bun install --frozen-lockfile --ignore-scripts` still
// requests packages from a bunfig.toml-configured registry rather than the
// default one (globally, per scope under [install.scopes], and per scope via
// [install]'s "scopes" key), and, more severely, that a redirected
// [install.cache] dir makes the same frozen install silently succeed with
// attacker-controlled file content from that directory -- no network fetch,
// no integrity-check failure, and no lifecycle script needed. bunfig.toml's
// "preload" entry was verified empirically not to fire on `bun install` at all
// (only on `bun run`/the bun runtime), so it is not treated as a hazard
// here. Like detectNpmrcHazard, a bunfig.toml that exists but cannot be read
// is itself a hazard, distinct from no bunfig.toml existing at all. This is
// a narrow, section-and-key scan for the hazardous forms above (registry and
// scopes only; cache is left to the backstop below for its detail text),
// not a general TOML parser -- consistent with detectNpmrcHazard's own
// narrow ini scan of .npmrc. Because it is not a real TOML parser, it cannot
// recognize every legal spelling that redirects install behavior (a header
// followed by a trailing comment, a leading UTF-8 BOM, or an inline table
// were all verified empirically against real Bun 1.3.11 to be honored while
// evading the structured scan below) -- and a keyword-by-keyword backstop
// will always be one Bun release behind the next [install]-namespaced key.
// So the fail-closed backstop matches a union of "install", "registry",
// "scopes", and "cache" anywhere in the file (case-insensitive), at the cost
// of over-refusing rather than under-refusing. Matching only "install" was
// tried and found insufficient on its own: real Bun 1.3.11 decodes TOML
// escape sequences -- \uXXXX, \xHH, and octal \NNN alike -- inside a quoted
// table name or a quoted key (["insta\x6Cl"]/"regist\x72y" decodes the same
// way ["install"]/"registry" would), which can strip every one of these
// keywords from a substring scan while the redirect -- including into
// [install.cache], the cache-poisoning vector above -- still takes effect.
// Rather than keep enumerating individual escape spellings, any backslash
// anywhere in the file is treated as its own fail-closed ground for refusal:
// every TOML escape mechanism, present or future, requires a backslash to
// invoke, and a legitimate bunfig.toml's paths and settings have no
// legitimate reason to contain one. Together these narrow the gap but do not
// close it for every possible future [install]-namespaced key or every
// other TOML encoding trick; the structured cases above remain only for
// their more specific, actionable detail text.
func detectBunfigHazard(root string) string {
	path := filepath.Join(root, "bunfig.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		return bunfigReadHazard(path)
	}
	data = bytes.TrimPrefix(data, []byte("\xEF\xBB\xBF"))
	if reason := bunfigInstallRedirect(data); reason != "" {
		return reason
	}
	return bunfigUnverifiedRedirect(data)
}

func bunfigReadHazard(path string) string {
	if _, statErr := os.Lstat(path); errors.Is(statErr, fs.ErrNotExist) {
		return ""
	}
	return "committed bunfig.toml could not be read"
}

func bunfigInstallRedirect(data []byte) string {
	section := ""
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			section = bunfigTableName(line)
			continue
		}
		if reason := bunfigInstallRedirectLine(section, line); reason != "" {
			return reason
		}
	}
	return ""
}

func bunfigTableName(line string) string {
	// A table header's name runs up to its closing ']', not to the
	// end of the line -- real Bun 1.3.11 also honors a trailing
	// comment after the header (`[install] # hi`), which
	// strings.HasSuffix(line, "]") would reject outright, leaving
	// section unset and the redirect below unseen. TOML also
	// permits whitespace inside the brackets ([ install ]) and a
	// quoted table name (["install"]); both are honored the same
	// way and handled by the same Trim below.
	name, _, _ := strings.Cut(line[1:], "]")
	return strings.ToLower(strings.Trim(strings.TrimSpace(name), `"'`))
}

func bunfigInstallRedirectLine(section, line string) string {
	key, value, found := strings.Cut(line, "=")
	if !found {
		return ""
	}
	key = strings.ToLower(strings.TrimSpace(key))
	value = trimConfigValueQuotes(strings.TrimSpace(value))
	switch {
	case section == "install" && key == "registry":
		return "committed bunfig.toml redirects the install registry (registry=" + value + ")"
	case section == "install" && key == "scopes":
		return "committed bunfig.toml redirects scoped install registries (scopes=" + value + ")"
	case section == "install.scopes":
		return "committed bunfig.toml redirects a scoped install registry (" + key + "=" + value + ")"
	}
	return ""
}

func bunfigUnverifiedRedirect(data []byte) string {
	const unverified = "committed bunfig.toml could not be verified to leave package resolution unredirected"
	// Every TOML escape sequence (\uXXXX, \xHH, octal \NNN, and any future
	// form Bun adds) requires a backslash to invoke, in either a quoted
	// table name or a quoted key -- a general check for the mechanism, not
	// an enumeration of its spellings. A legitimate bunfig.toml's forward-
	// slash paths and settings never need one.
	if strings.Contains(string(data), `\`) {
		return unverified
	}
	lower := strings.ToLower(string(data))
	for _, keyword := range []string{"install", "registry", "scopes", "cache"} {
		if strings.Contains(lower, keyword) {
			return unverified
		}
	}
	return ""
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
