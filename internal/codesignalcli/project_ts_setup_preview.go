package codesignalcli

import (
	"errors"
	"fmt"
	"time"
)

// SetupPreviewTimeout bounds how long a project-package setup command may
// run once a customer confirms it (AC-SET-2). BuildSetupPreview only
// discloses this bound; enforcing it during execution is a later task's
// concern.
const SetupPreviewTimeout = 5 * time.Minute

// setupCommandTemplate is one package-manager kind's frozen, non-executing
// argv template (SA-280-012): the bare executable name -- never a resolved
// absolute path -- plus its arguments and the truthful on-disk-effect
// disclosure for AC-SET-2. None of the three frozen rows can rewrite a
// lockfile: npm ci and pnpm/bun's --frozen-lockfile install both fail
// instead of writing one. What they do mutate is node_modules -- expected
// Changes must disclose that, not a lockfile rewrite that cannot happen.
type setupCommandTemplate struct {
	executable              string
	args                    []string
	expectedChanges         string
	scriptSuppressionPolicy string
}

// setupCommandTemplates keys off the same manager-kind constants
// checkPackageManager classifies against (project_ts_setup_matrix.go),
// rather than re-typing a second copy of the frozen command strings.
//
// pnpm's row also carries --ignore-pnpmfile: --ignore-scripts alone does
// not stop pnpm from loading and running a committed .pnpmfile.cjs (module
// top level and any hooks.readPackage hook) during install -- that is a
// separate opt-out, and detectPackageManagerHazard does not vet pnpm
// configuration, so this row is pnpm's only line of defense against it.
// Bun's analogous vector, a bunfig.toml "preload" entry, was verified not
// to fire on `bun install` (only on `bun run`/the bun runtime), so bun's
// row needs no equivalent flag.
var setupCommandTemplates = map[string]setupCommandTemplate{
	packageManagerKindNPM: {
		executable: "npm",
		args:       []string{"ci", "--ignore-scripts"},
		expectedChanges: "removes and recreates node_modules/ in the working directory from package-lock.json; " +
			"leaves package-lock.json and package.json unchanged -- npm ci fails instead if they disagree",
		scriptSuppressionPolicy: "--ignore-scripts suppresses this package manager's lifecycle scripts (including dependency build scripts) for this run",
	},
	packageManagerKindPNPM: {
		executable: "pnpm",
		args:       []string{"install", "--frozen-lockfile", "--ignore-scripts", "--ignore-pnpmfile"},
		expectedChanges: "installs packages into node_modules/ in the working directory (and pnpm's content-addressable store); " +
			"leaves pnpm-lock.yaml and package.json unchanged -- fails instead of changing them",
		scriptSuppressionPolicy: "--ignore-scripts suppresses this package manager's lifecycle scripts (including dependency build scripts) for this run; " +
			"--ignore-pnpmfile additionally skips loading and running a committed .pnpmfile.cjs, which pnpm would otherwise execute (including its hooks.readPackage hook) during install regardless of --ignore-scripts",
	},
	packageManagerKindBun: {
		executable: "bun",
		args:       []string{"install", "--frozen-lockfile", "--ignore-scripts"},
		// Bun recognizes two lockfile variants (bun.lock, bun.lockb; see
		// packageManagerLockfileBasenames) and BuildSetupPreview is not told
		// which one this repository has, so this disclosure names neither --
		// naming one would risk citing a file that does not exist here.
		expectedChanges: "installs packages into node_modules/ in the working directory (and bun's install cache); " +
			"leaves the lockfile and package.json unchanged -- disallows any lockfile changes",
		scriptSuppressionPolicy: "--ignore-scripts suppresses this package manager's lifecycle scripts (including dependency build scripts) for this run",
	},
}

// SetupPreview is the disclosure Coach must show before executing a
// project-package setup choice (AC-SET-2): the exact command that will run,
// where it will run, what it may change on disk, whether it reaches the
// network, its lifecycle-script policy, and its bounded timeout. Building a
// SetupPreview never spawns a process.
type SetupPreview struct {
	Executable              string
	Args                    []string
	WorkingDirectory        string
	ExpectedChanges         string
	NetworkDisclosure       string
	ScriptSuppressionPolicy string
	Timeout                 time.Duration
}

// ErrSetupPreviewUnavailable reports that choice carries no frozen preview:
// either it is not a project-package choice, or packageManagerKind names a
// manager outside the frozen adapter matrix (SA-280-012). Fail-closed: an
// unrecognized kind never falls back to a guessed command.
var ErrSetupPreviewUnavailable = errors.New("setup preview: no frozen command template for this choice")

// BuildSetupPreview renders the pre-execution disclosure for choice,
// resolved against packageManagerKind -- the same kind checkPackageManager
// already classified onto checks.package_manager -- and workingDirectory,
// the directory containing the manifest that owns the selected origin. It
// performs no filesystem or network access and executes nothing.
func BuildSetupPreview(choice SetupChoice, packageManagerKind, workingDirectory string) (SetupPreview, error) {
	if choice.Kind != SetupChoiceProjectPackage {
		return SetupPreview{}, fmt.Errorf("%w: choice kind %q", ErrSetupPreviewUnavailable, choice.Kind)
	}
	template, ok := setupCommandTemplates[packageManagerKind]
	if !ok {
		return SetupPreview{}, fmt.Errorf("%w: package manager kind %q", ErrSetupPreviewUnavailable, packageManagerKind)
	}

	return SetupPreview{
		Executable:              template.executable,
		Args:                    append([]string(nil), template.args...),
		WorkingDirectory:        workingDirectory,
		ExpectedChanges:         template.expectedChanges,
		NetworkDisclosure:       "may reach the configured package registry over the network to download or verify dependencies",
		ScriptSuppressionPolicy: template.scriptSuppressionPolicy,
		Timeout:                 SetupPreviewTimeout,
	}, nil
}
