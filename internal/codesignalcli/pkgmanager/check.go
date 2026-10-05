// Package pkgmanager detects which package manager (npm, pnpm, Bun, or yarn) a
// TypeScript project declares, from the worktree or a committed revision,
// probes the version on PATH, and rejects configuration that would redirect
// package resolution.
package pkgmanager

import (
	"os"
	"path/filepath"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

// Check detects and classifies the package manager of every
// selected root against the frozen adapter support matrix (SA-280-012),
// reading worktree state at the same per-root context tstoolchain.ResolveCompiler
// resolves a compiler against. No install is ever executed here -- an
// ambiguous, unverifiable, or hazardous finding fails closed with a distinct
// gap code (SA-280-015) instead.
//
// The version that classifies the row is probed from the manager binary that
// would actually run (ProbeVersion), never taken from
// package.json's packageManager pin. The frozen adapter rows invoke the bare
// executable name and Coach never installs or switches to a pinned release,
// so a pin describes an intention while the probe describes what will run;
// classifying by the pin would clear a row for a binary that is not there.
// The pin is still recorded, and BuildSetupPreview discloses it whenever it
// disagrees with the probe.
//
// A manifest that Contexts actually resolves -- whether under
// a validated root or, absent one, the default "." context's own nearest
// package.json -- is real evidence and is classified regardless of
// policyPassed, the same way checkProjectShape trusts a root-level
// package.json unconditionally. But when no context resolves any manifest
// at all, Contexts falls back to classifying whatever bare
// metadata sits at the worktree root, a directory a genuine monorepo's
// policy may never actually select. Without a validated policy that
// fallback is unjustified, so policyPassed false skips it and reports
// not_checked instead of a package_manager_* gap this check has no real
// root context to justify (R1, mirroring checkProjectShape's own gate).
func Check(dir string, roots []string, policyPassed bool) projectreadiness.Check {
	contexts, fellBackToWorktreeRoot := Contexts(tstoolchain.WorktreeRoot(dir), roots)
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
	if detection.kind == KindYarn {
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
		if detail := DetectPackageManagerHazard(packageDir, detection.kind); detail != "" {
			return packageManagerConfigUnverifiable(detection, detail)
		}
		if detail := requireReadableLockfile(packageDir, detection.kind); detail != "" {
			return packageManagerConfigUnverifiable(detection, detail)
		}
	}
	return classifyProbedPackageManagerVersion(detection)
}

func packageManagerConfigUnverifiable(detection managerDetection, detail string) projectreadiness.Check {
	return projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapPackageManagerConfigUnverifiable, Kind: detection.kind, PinnedVersion: detection.pin, Detail: detail}
}

// requireReadableLockfile reports a hazard detail unless at least one of
// kind's recognized lockfile basenames (SA-280-012) exists and can be read
// at root: none of the matrix-recognized managers' locked install argv can
// run without one. A basename that is missing and one that exists but
// cannot be read are treated identically -- both fail closed -- unlike
// detectNpmrcHazard, where absence of the (optional) file is itself safe.
func requireReadableLockfile(root, kind string) string {
	for basename, k := range LockfileBasenames {
		if k != kind {
			continue
		}
		if _, err := os.ReadFile(filepath.Join(root, basename)); err == nil {
			return ""
		}
	}
	return "no readable " + kind + " lockfile was found (SA-280-012)"
}
