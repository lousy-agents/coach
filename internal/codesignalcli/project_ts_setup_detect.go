package codesignalcli

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
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
func checkPackageManager(dir string, roots []string, policyPassed bool) projectreadiness.Check {
	contexts, fellBackToWorktreeRoot := packageManagerContexts(compilerWorktreeRoot(dir), roots)
	if fellBackToWorktreeRoot && !policyPassed {
		return projectreadiness.Check{State: projectreadiness.NotChecked}
	}

	detection, ok := detectPackageManagerAcrossContexts(contexts)
	if !ok {
		return projectreadiness.Check{State: projectreadiness.NotChecked}
	}
	if detection.ambiguous {
		return projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapPackageManagerAmbiguous}
	}
	// Yarn has no matrix row and is withheld before hazard/version checks
	// are consulted.
	if detection.kind == packageManagerKindYarn {
		return projectreadiness.Check{
			State:         projectreadiness.Fail,
			Code:          projectreadiness.GapPackageManagerVersionUnsupported,
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

func packageManagerConfigUnverifiable(detection packageManagerDetection, detail string) projectreadiness.Check {
	return projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapPackageManagerConfigUnverifiable, Kind: detection.kind, PinnedVersion: detection.pin, Detail: detail}
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
	return reconcilePackageManagerDetection(fieldKind, fieldVersion, fieldOK, lockKind, lockOK, lockAmbiguous)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

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

// isNpmrcRegistryKey reports whether key is .npmrc's global "registry"
// setting or a scoped "@scope:registry" override. Shared between npm's
// detectNpmrcHazard and detectNpmrcRegistryHazard (pnpm and Bun), since both
// read the same .npmrc format npm does for their own registry resolution.
func isNpmrcRegistryKey(key string) bool {
	return key == "registry" || strings.HasSuffix(key, ":registry")
}

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

// Every TOML escape sequence (\uXXXX, \xHH, octal \NNN, and any future
// form Bun adds) requires a backslash to invoke, in either a quoted
// table name or a quoted key -- a general check for the mechanism, not
// an enumeration of its spellings. A legitimate bunfig.toml's forward-
// slash paths and settings never need one.
