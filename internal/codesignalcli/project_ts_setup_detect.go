package codesignalcli

import (
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
//
// A manifest that packageManagerContexts actually resolves -- whether under
// a validated root or, absent one, the default "." context's own nearest
// package.json -- is real evidence and is classified regardless of
// policyPassed, the same way checkProjectShape trusts a root-level
// package.json unconditionally. But when no context resolves any manifest
// at all, packageManagerContexts falls back to classifying whatever bare
// metadata sits at the worktree root, a directory a genuine monorepo's
// policy may never actually select. Without a validated policy that
// fallback is unjustified, so policyPassed false skips it and reports
// not_checked instead of a package_manager_* gap this check has no real
// root context to justify (R1, mirroring checkProjectShape's own gate).
func checkPackageManager(dir string, roots []string, policyPassed bool) ReadinessCheck {
	contexts, fellBackToWorktreeRoot := packageManagerContexts(compilerWorktreeRoot(dir), roots)
	if fellBackToWorktreeRoot && !policyPassed {
		return ReadinessCheck{State: ReadinessNotChecked}
	}

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
// package.json is still classified rather than silently unchecked --
// fellBack reports when that stand-in fired, so a caller without a
// validated policy can tell that classification apart from one resolveCompiler
// actually found a manifest for (checkPackageManager's own R1 gate).

// detectPackageManagerAcrossContexts reduces the selected roots' package
// contexts to one detection. Contexts naming different managers are reported
// as ambiguous rather than resolved to whichever root was listed first: the
// frozen rows install one manager in one working directory, so there is no
// honest way to serve two. A context with no recognized metadata contributes
// nothing and is not itself a disagreement. ok is false only when no context
// recognized anything at all.

func packageManagerConfigUnverifiable(detection packageManagerDetection, detail string) ReadinessCheck {
	return ReadinessCheck{State: ReadinessFail, Code: GapPackageManagerConfigUnverifiable, Kind: detection.kind, PinnedVersion: detection.pin, Detail: detail}
}

type packageManagerDetection struct {
	kind      string
	pin       string // package.json's packageManager version, recorded for disclosure only
	ambiguous bool
}

// reconcilePackageManagerDetection is the single cross-check rule shared by
// the worktree and snapshot detection paths so they cannot diverge (SA-280-012).

// detectPackageManager identifies the repository's package manager from its
// recognized metadata (SA-280-012): a package.json "packageManager" pin and a
// bare lockfile are equally good evidence of which manager a repository uses,
// and they are cross-checked against each other rather than ranked. ok is
// false only when no recognized metadata exists at all, distinct from
// detection.ambiguous (recognized metadata that disagrees with itself).
func detectPackageManager(root string) (packageManagerDetection, bool) {
	fieldKind, fieldVersion, fieldOK := readPackageManagerField(root)
	lockKind, lockAmbiguous, lockOK := detectPackageManagerLockfile(root)
	return reconcilePackageManagerDetection(fieldKind, fieldVersion, fieldOK, lockKind, lockOK, lockAmbiguous)
}

// readPackageManagerField reads package.json's Corepack-style
// "packageManager": "<name>@<version>" field. An unrecognized manager name
// is treated the same as an absent field, falling back to lockfile
// detection instead.

// detectPackageManagerLockfile reports the manager kind implied by a
// recognized lockfile basename present at root. More than one distinct
// kind's lockfile committed simultaneously is reported as ambiguous rather
// than picking one arbitrarily.

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// detectPackageManagerHazard reports a non-empty detail describing a
// repository-controlled configuration hazard from kind's Hazards column
// (SA-280-012), or "" if none. The lockfile-readability precondition itself
// is kind-agnostic; see requireReadableLockfile.

// Bun resolves its install registry from a committed .npmrc the same
// way npm/pnpm do, on top of bunfig.toml rather than instead of it
// (verified empirically against real Bun 1.3.11: `bun install
// --ignore-scripts` still fails with ConnectionRefused against a
// .npmrc-redirected host with no bunfig.toml present at all) -- so
// both must be checked, not just bunfig.toml.

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

// readCommittedBunfig reads root's bunfig.toml. present is false both when
// there is genuinely no file (detail "": nothing to be hazardous) and when
// one exists but cannot be read (detail names the hazard) -- the fail-closed
// half AC-SET-12 requires, since an unreadable config is not a safe one.

// bunfigRedirectHazard names the redirection it can read directly out of the
// file, walking it as Bun does rather than parsing TOML: a section header
// followed by key/value lines.

// bunfigSectionName reads a table header's name. The name runs up to its
// closing ']', not to the end of the line -- real Bun 1.3.11 also honors a
// trailing comment after the header (`[install] # hi`), which
// strings.HasSuffix(line, "]") would reject outright, leaving the section
// unset and a redirect below it unseen. TOML also permits whitespace inside
// the brackets ([ install ]) and a quoted table name (["install"]); both are
// honored the same way by the same Trim.
func bunfigSectionName(line string) string {
	name, _, _ := strings.Cut(line[1:], "]")
	return strings.ToLower(strings.Trim(strings.TrimSpace(name), `"'`))
}

// bunfigUnverifiableHazard fails closed on a file this reader cannot claim to
// have understood, rather than on a redirect it recognized.

// Every TOML escape sequence (\uXXXX, \xHH, octal \NNN, and any future
// form Bun adds) requires a backslash to invoke, in either a quoted
// table name or a quoted key -- a general check for the mechanism, not
// an enumeration of its spellings. A legitimate bunfig.toml's forward-
// slash paths and settings never need one.

// trimConfigValueQuotes strips a single layer of matching double or single
// quotes from a config value, per .npmrc's ini quoting rules -- also
// sufficient for a TOML basic/literal string's outer quotes in
// detectBunfigHazard's narrow scan.
