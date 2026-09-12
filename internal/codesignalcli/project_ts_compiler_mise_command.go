package codesignalcli

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// miseInstallTimeout bounds `mise install`. It is far longer than
// miseProbeTimeout because installation does real network I/O (fetching a
// package); the read-only probes in project_ts_compiler_mise_probe.go never
// do.
const miseInstallTimeout = 5 * time.Minute

// miseNpmScriptSuppressionEnvKey/Value apply the npm row's suppression
// (miseBackingNpmSuppressionFlag, "--ignore-scripts") to the npm invocation
// mise's npm backend may spawn internally, via npm's own env-var config
// convention (npm_config_<key>) rather than a CLI flag: mise's npm backend
// exposes no documented flag-passthrough mechanism for that internal
// invocation.
//
// Verified empirically against mise 2026.9.5 (both branches, with this env
// var present and absent): mise's default npm backend (its own built-in
// installer, active whenever the npm.shell_out setting is left at its
// default false) never executes a package's lifecycle scripts at all,
// regardless of this env var. When npm.shell_out is true, mise instead
// shells out to a real npm and its own debug log shows it unconditionally
// appends "--ignore-scripts=true" to that invocation itself, regardless of
// this env var too. This env var is therefore redundant with both of mise's
// current code paths, but pins Coach's own contract to a mechanism Coach
// applies directly rather than depending entirely on mise's present-day
// defaults, which the frozen row's own AC-4 gap code exists to guard
// against if a future mise release ever changes them.
const miseNpmScriptSuppressionEnvKey = "npm_config_ignore_scripts"
const miseNpmScriptSuppressionEnvValue = "true"

// runMiseInstallInsulated runs `mise install <toolSpec>` confined the same
// way runMiseProbe confines a read-only probe: a private per-call working
// directory outside any repository (so a repository-controlled mise.toml,
// including one carrying an env `exec()` template that
// hasMiseConfigHazard's own scan does not flag, is never discovered by mise
// walking up from cwd), and an environment limited to
// PATH/HOME/MISE_DATA_DIR/MISE_CONFIG_DIR -- plus the npm lifecycle-script
// suppression above.
//
// toolSpec is the full mise tool identifier ("npm:<pkg>@<version>" or, for a
// local fixture, "npm:file:<path>"). Production callers below always pass
// the frozen row's exact npm:typescript@<version> (AC-10); toolSpec stays a
// parameter so the exact subprocess-invocation code every production
// install goes through can be proven, with a local fixture package, without
// ever inventing a different production command.
//
// attempted is true once the subprocess actually started (cmd.Start()
// succeeded) -- the AC-SET-7 boundary: local disk state may already differ
// from that point on, whether or not the outcome could be observed. It is
// false whenever the subprocess never started: mise absent from PATH, or
// the private working directory could not be created -- insulationFailed
// distinguishes the latter (AC-16's insulation guarantee itself could not be
// established) from the former, since only the latter is a genuine gap in
// mise's own configuration/environment rather than mise simply being
// absent. observed is true only when the subprocess's exit could actually
// be read back -- not a timeout, stdout-budget overflow, or read failure --
// and exitErr (the process's own non-zero exit) is meaningful only when
// observed is true.
func runMiseInstallInsulated(ctx context.Context, toolSpec string) (attempted, observed, insulationFailed bool, exitErr error) {
	if _, lookErr := exec.LookPath("mise"); lookErr != nil {
		return false, false, false, nil
	}
	workDir, tempErr := os.MkdirTemp("", "coach-mise-install-")
	if tempErr != nil {
		return false, false, true, nil
	}
	defer func() { _ = os.RemoveAll(workDir) }()

	env := append(append([]string{}, miseProbeEnv()...), miseNpmScriptSuppressionEnvKey+"="+miseNpmScriptSuppressionEnvValue)
	attempted, observed, exitErr = runBoundedMiseInstallSubprocess(ctx, workDir, env, toolSpec)
	return attempted, observed, false, exitErr
}

// runBoundedMiseInstallSubprocess mirrors runBoundedSubprocessProbeAt's
// timeout/output-budget confinement (project_readiness_probe.go), but that
// shared probe helper collapses every post-Start failure into a single
// opaque error and so cannot answer whether the subprocess actually started
// -- exactly the distinction runMiseInstallInsulated's AC-SET-7 contract
// needs. This keeps that answer local to the one caller that needs it
// rather than changing the shared read-only-probe helper's contract for
// every other probe in this package.
func runBoundedMiseInstallSubprocess(ctx context.Context, dir string, env []string, toolSpec string) (attempted, observed bool, exitErr error) {
	ctx, cancel := context.WithTimeout(ctx, miseInstallTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "mise", "install", toolSpec)
	cmd.Dir = dir
	cmd.Env = env
	stdout, pipeErr := cmd.StdoutPipe()
	if pipeErr != nil {
		return false, false, nil
	}
	if startErr := cmd.Start(); startErr != nil {
		return false, false, nil
	}

	data, readErr := io.ReadAll(io.LimitReader(stdout, maxMiseProbeOutput+1))
	waitErr := cmd.Wait()

	// Checked ahead of readErr/waitErr for the same reason
	// runBoundedSubprocessProbeAt does: killing the child on deadline makes
	// both of those non-nil too, but the deadline is the true cause.
	if ctx.Err() == context.DeadlineExceeded {
		return true, false, nil
	}
	if readErr != nil {
		return true, false, nil
	}
	if int64(len(data)) > maxMiseProbeOutput {
		return true, false, nil
	}
	return true, true, waitErr
}

// miseInstallToolSpec derives the frozen row's exact install argument
// (miseInstallCommandTemplate) rather than re-concatenating
// miseNpmTypescriptTool and version separately, so this file cannot drift
// from the frozen template if it ever changes shape.
func miseInstallToolSpec(version string) string {
	return strings.TrimPrefix(miseInstallCommand(version), "mise install ")
}

// miseInstallResult reports the outcome of installMiseTypescriptProject or
// installMiseTypescriptGlobal.
type miseInstallResult struct {
	// Trusted is false when the matching trust gate
	// (evaluateMiseProjectTrust/evaluateMiseGlobalTrust) refused before
	// anything ran; Code then names the refusal and every field below is
	// zero.
	Trusted bool

	// Attempted is true once the `mise install` subprocess actually started
	// (cmd.Start() succeeded), whether or not it exited 0, timed out, or its
	// output could not be read back -- the AC-SET-7 "identify files that may
	// have changed" precondition: local disk state may already differ even
	// when Succeeded is false.
	Attempted bool

	// Observed is true only when the subprocess's exit could actually be
	// read back (mirroring runMiseInstallInsulated's own observed return).
	// A false Succeeded with Observed true and a non-empty Class means the
	// subprocess itself exited 0 and only post-install eligibility
	// verification failed -- a materially different outcome from Observed
	// being false (timeout/output-budget overflow) or the subprocess's own
	// exit being non-zero, both of which leave Class empty.
	Observed bool

	// Succeeded is true only when install, `mise where`, and
	// classifyCompilerCandidate all agree the freshly-installed TypeScript
	// is compilerClassEligible -- the AC-SET-6 rerun-readiness precondition.
	Succeeded bool

	// Code names why Trusted or Succeeded is false: the trust gate's gap
	// code when Trusted is false, or GapPackageManagerConfigUnverifiable
	// when the install's own insulation (its private working directory)
	// could not be established, before any subprocess ever started. A false
	// Succeeded after a true Attempted has no frozen gap code of its own
	// (the decision table in project_ts_compiler_mise.go classifies origin
	// resolution, not a just-run install); Class below carries that detail
	// instead, and Code stays empty in that case.
	Code string

	// Origin, Version, Path, NativePath, and Class mirror
	// classifyCompilerCandidate's result for the freshly-installed
	// location, populated once the install's outcome was observed and
	// exited zero.
	Origin     string
	Version    string
	Path       string
	NativePath string
	Class      string
}

// installMiseTypescriptProject installs the frozen row's npm:typescript tool
// at version, gated on the project mise scope's trust
// (evaluateMiseProjectTrust). See installMiseTypescript for the shared
// install/verify sequence.
func installMiseTypescriptProject(ctx context.Context, worktreeRoot, version string) miseInstallResult {
	return installMiseTypescript(ctx, compilerOriginMiseProject, version, evaluateMiseProjectTrust(ctx, worktreeRoot))
}

// installMiseTypescriptGlobal installs the frozen row's npm:typescript tool
// at version, gated on the global mise scope's trust
// (evaluateMiseGlobalTrust). See installMiseTypescript for the shared
// install/verify sequence.
func installMiseTypescriptGlobal(ctx context.Context, version string) miseInstallResult {
	return installMiseTypescript(ctx, compilerOriginMiseGlobal, version, evaluateMiseGlobalTrust(ctx))
}

// installMiseTypescript runs the frozen row's command sequence: refuse
// unless trust already verified this scope, then `mise install`, then
// (mirroring every other mise-origin resolution in this package)
// classifyCompilerCandidate against a `mise where`-backed locator to confirm
// the installed compiler is genuinely eligible rather than merely
// exit-code-zero.
func installMiseTypescript(ctx context.Context, origin, version string, trust miseSetupTrust) miseInstallResult {
	if !trust.trusted {
		return miseInstallResult{Code: trust.code}
	}
	attempted, observed, insulationFailed, exitErr := runMiseInstallInsulated(ctx, miseInstallToolSpec(version))
	result := miseInstallResult{Trusted: true, Attempted: attempted, Observed: observed}
	if insulationFailed {
		result.Code = GapPackageManagerConfigUnverifiable
		return result
	}
	if !observed || exitErr != nil {
		return result
	}

	candidate := classifyCompilerCandidate(origin, func() (string, bool) {
		pkgDir, ok := locateMiseTypescriptInstall(ctx, version)
		if ok {
			hoistMiseNativeTypescriptNextTo(pkgDir)
		}
		return pkgDir, ok
	})
	result.Origin = candidate.origin
	result.Version = candidate.version
	result.Path = candidate.path
	result.NativePath = candidate.nativePath
	result.Class = candidate.class
	result.Succeeded = candidate.class == compilerClassEligible
	return result
}

// hoistMiseNativeTypescriptNextTo repairs a gap verified empirically against
// mise 2026.9.5 (its default npm backend, internally "aube" 2.2.13): the
// installed npm:typescript package itself is hoisted from mise's internal
// package store to a symlink at pkgDir (<toolRoot>/node_modules/typescript)
// -- the same shape a local `npm install typescript` produces -- but the
// platform-native @typescript/typescript-<os>-<arch> package that
// typescript declares as an optionalDependency is not hoisted the same way.
// It is genuinely installed and fully correct, reachable as a sibling of
// typescript's own real (symlink-resolved) directory, but no matching
// symlink exists as a sibling of pkgDir itself -- the exact place
// resolveNativePackage (project_ts_native.go) looks for it. This locates the
// already-installed native package next to typescript's real location and
// completes the same hoisting mise's own backend already performs for
// typescript, without altering the frozen `mise install`/`mise where`
// invocations or duplicating resolveNativePackage's own path logic. A
// symlink that already exists and reads back correctly at pkgDir's sibling
// (e.g. a local, non-mise npm install, or a future mise release that hoists
// correctly on its own) is left untouched; any failure here is silent and
// non-fatal -- classifyCompilerCandidate's own read of the expected sibling
// path is still the single source of truth for eligibility.
func hoistMiseNativeTypescriptNextTo(pkgDir string) {
	expected := nativePackageDirNextTo(pkgDir)
	if _, exists, unreadable := readTypescriptVersionAt(expected); exists && !unreadable {
		return
	}
	resolved, err := filepath.EvalSymlinks(pkgDir)
	if err != nil || resolved == pkgDir {
		return
	}
	real := nativePackageDirNextTo(resolved)
	if _, exists, unreadable := readTypescriptVersionAt(real); !exists || unreadable {
		return
	}
	if err := os.MkdirAll(filepath.Dir(expected), 0o755); err != nil {
		return
	}
	_ = os.Symlink(real, expected)
}
