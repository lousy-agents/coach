package claudehooks

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestSetupMise_UpgradesStaleVersion verifies that the SessionStart hook
// installs the mise version pinned in mise.toml when an older mise binary is
// already on PATH. This is an acceptance test for .claude/hooks/setup-mise.sh.
func TestSetupMise_UpgradesStaleVersion(t *testing.T) {
	tmp, home, project, bin, npmDir, localBin := setupTestDirs(t)

	// Stale mise already on PATH.
	oldMise := filepath.Join(bin, "mise")
	if err := os.WriteFile(oldMise, []byte(fakeMiseScript("2024.1.1", "/old/mise/bin")), 0755); err != nil {
		t.Fatal(err)
	}

	// Fake npm records its invocation and writes a newer mise binary.
	npmCalled := filepath.Join(tmp, "npm-called")
	newMise := filepath.Join(localBin, "mise")
	npmBin := filepath.Join(npmDir, "npm")
	if err := os.WriteFile(npmBin, []byte(fakeNpmScript(npmCalled, newMise, localBin)), 0755); err != nil {
		t.Fatal(err)
	}

	envFile := filepath.Join(tmp, "env")
	runHook(t, home, project, envFile, npmDir+":"+bin)

	if _, err := os.Stat(npmCalled); err != nil {
		t.Fatalf("npm was not invoked to upgrade the stale mise binary: %v", err)
	}
	args, _ := os.ReadFile(npmCalled)
	if !strings.Contains(string(args), "mise@2026.7.7") {
		t.Fatalf("npm install did not target the expected mise version: %s", args)
	}
}

func TestSetupMise_SkipsInstallWhenCurrent(t *testing.T) {
	tmp, home, project, bin, npmDir, localBin := setupTestDirs(t)

	// Current mise already on PATH, with the same version format reported by the real binary.
	currentMise := filepath.Join(bin, "mise")
	if err := os.WriteFile(currentMise, []byte(fakeMiseScript("mise 2026.7.7 linux-x64", localBin)), 0755); err != nil {
		t.Fatal(err)
	}

	// Fake npm should not be invoked.
	npmCalled := filepath.Join(tmp, "npm-called")
	npmBin := filepath.Join(npmDir, "npm")
	if err := os.WriteFile(npmBin, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > "+npmCalled+"\n"), 0755); err != nil {
		t.Fatal(err)
	}

	envFile := filepath.Join(tmp, "env")
	runHook(t, home, project, envFile, npmDir+":"+bin)

	if _, err := os.Stat(npmCalled); err == nil {
		t.Fatalf("npm was invoked even though the current mise version satisfies min_version")
	}
}

// TestSetupMise_PersistsToolBinPaths verifies that the hook persists the active
// tool paths through CLAUDE_ENV_FILE so that later Bash commands can use mise
// and the pinned tools without activation.
func TestSetupMise_PersistsToolBinPaths(t *testing.T) {
	tmp, home, project, bin, npmDir, localBin := setupTestDirs(t)

	goBin := filepath.Join(tmp, "go-install", "bin")
	nodeBin := filepath.Join(tmp, "node-install", "bin")
	for _, d := range []string{goBin, nodeBin} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}

	currentMise := filepath.Join(bin, "mise")
	binPaths := strings.Join([]string{goBin, nodeBin}, "\n")
	if err := os.WriteFile(currentMise, []byte(fakeMiseScript("2026.7.7", binPaths)), 0755); err != nil {
		t.Fatal(err)
	}

	npmCalled := filepath.Join(tmp, "npm-called")
	npmBin := filepath.Join(npmDir, "npm")
	if err := os.WriteFile(npmBin, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > "+npmCalled+"\n"), 0755); err != nil {
		t.Fatal(err)
	}

	envFile := filepath.Join(tmp, "env")
	runHook(t, home, project, envFile, npmDir+":"+bin)

	if _, err := os.Stat(npmCalled); err == nil {
		t.Fatalf("npm was invoked even though a current mise version is on PATH")
	}

	data, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatalf("expected CLAUDE_ENV_FILE to be written: %v", err)
	}
	exported := string(data)
	if !strings.Contains(exported, localBin) {
		t.Fatalf("expected CLAUDE_ENV_FILE to prepend user local bin path; got:\n%s", exported)
	}
	for _, p := range []string{goBin, nodeBin} {
		if !strings.Contains(exported, p) {
			t.Fatalf("expected CLAUDE_ENV_FILE to contain tool bin path %q; got:\n%s", p, exported)
		}
	}
}

// TestSetupMise_StdoutSilentDuringInstall verifies that the hook stays silent on
// stdout on the fresh-install path, where npm is actually invoked. This is the
// path a first cloud session takes, so npm's progress output must not reach
// stdout and get injected into the conversation context.
func TestSetupMise_StdoutSilentDuringInstall(t *testing.T) {
	tmp, home, project, bin, npmDir, localBin := setupTestDirs(t)

	// No mise on PATH, so the hook must install it via npm.
	npmCalled := filepath.Join(tmp, "npm-called")
	newMise := filepath.Join(localBin, "mise")
	npmBin := filepath.Join(npmDir, "npm")
	noisyNpm := fakeNpmScript(npmCalled, newMise, localBin) +
		"echo 'added 1 package in 3s'\necho 'npm notice: something' >&2\n"
	if err := os.WriteFile(npmBin, []byte(noisyNpm), 0755); err != nil {
		t.Fatal(err)
	}

	envFile := filepath.Join(tmp, "env")
	stdout, stderr, err := runHookSplit(t, home, project, envFile, npmDir+":"+bin)
	if err != nil {
		t.Fatalf("setup-mise.sh failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}

	if _, err := os.Stat(npmCalled); err != nil {
		t.Fatalf("expected npm to be invoked on the fresh-install path: %v", err)
	}
	if len(bytes.TrimSpace(stdout)) != 0 {
		t.Fatalf("expected empty stdout while installing mise; got: %q", stdout)
	}
}

// TestSetupMise_NoEmptyPathElement verifies that the hook never writes an empty
// element into PATH. An empty element resolves to the current working
// directory, which would let a file in the repo shadow a real command for every
// later Bash call.
func TestSetupMise_NoEmptyPathElement(t *testing.T) {
	tmp, home, project, bin, npmDir, localBin := setupTestDirs(t)

	// A current mise that reports no tool bin paths at all.
	currentMise := filepath.Join(bin, "mise")
	if err := os.WriteFile(currentMise, []byte(fakeMiseScript("2026.7.7", "")), 0755); err != nil {
		t.Fatal(err)
	}

	npmBin := filepath.Join(npmDir, "npm")
	if err := os.WriteFile(npmBin, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}

	envFile := filepath.Join(tmp, "env")
	runHook(t, home, project, envFile, npmDir+":"+bin)

	data, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatalf("expected CLAUDE_ENV_FILE to be written: %v", err)
	}
	exported := strings.TrimSpace(string(data))
	if !strings.Contains(exported, localBin) {
		t.Fatalf("expected CLAUDE_ENV_FILE to prepend user local bin path; got:\n%s", exported)
	}

	value := strings.TrimPrefix(exported, "export PATH=")
	for _, elem := range strings.Split(value, ":") {
		if elem == "" || elem == "''" || elem == `""` {
			t.Fatalf("PATH contains an empty element (resolves to cwd); got:\n%s", exported)
		}
	}
}

// TestSetupMise_StdoutSilentOnSuccess verifies that the hook produces no stdout
// on success. SessionStart stdout is injected into the conversation context, so
// install progress output must be redirected to stderr or discarded.
func TestSetupMise_StdoutSilentOnSuccess(t *testing.T) {
	tmp, home, project, bin, npmDir, localBin := setupTestDirs(t)

	currentMise := filepath.Join(bin, "mise")
	if err := os.WriteFile(currentMise, []byte(fakeMiseScriptWithNoise("2026.7.7", localBin)), 0755); err != nil {
		t.Fatal(err)
	}

	npmBin := filepath.Join(npmDir, "npm")
	if err := os.WriteFile(npmBin, []byte("#!/bin/sh\necho 'fake npm stdout'\necho 'fake npm stderr' >&2\n"), 0755); err != nil {
		t.Fatal(err)
	}

	envFile := filepath.Join(tmp, "env")
	stdout, stderr, err := runHookSplit(t, home, project, envFile, npmDir+":"+bin)
	if err != nil {
		t.Fatalf("setup-mise.sh failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}

	if len(bytes.TrimSpace(stdout)) != 0 {
		t.Fatalf("expected empty stdout on successful hook run; got: %q", stdout)
	}
}

// TestSetupMise_LocalNoOp verifies that the hook exits immediately when
// CLAUDE_CODE_REMOTE is not set to true, leaving local sessions unchanged.
func TestSetupMise_LocalNoOp(t *testing.T) {
	tmp, home, project, bin, npmDir, localBin := setupTestDirs(t)

	npmCalled := filepath.Join(tmp, "npm-called")
	npmBin := filepath.Join(npmDir, "npm")
	if err := os.WriteFile(npmBin, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > "+npmCalled+"\n"), 0755); err != nil {
		t.Fatal(err)
	}

	// CLAUDE_CODE_REMOTE explicitly cleared: os.Environ() may already carry
	// CLAUDE_CODE_REMOTE=true when this test itself runs inside a Claude Code
	// cloud session, which would otherwise mask the local no-op path.
	envFile := filepath.Join(tmp, "env")
	cmd := exec.Command("bash", absScript(t))
	cmd.Env = append(os.Environ(),
		"HOME="+home,
		"CLAUDE_CODE_REMOTE=",
		"CLAUDE_PROJECT_DIR="+project,
		"CLAUDE_ENV_FILE="+envFile,
	)
	cmd.Env = append(cmd.Env, "PATH="+npmDir+":"+bin+":/usr/bin:/bin")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("setup-mise.sh failed locally: %v\n%s", err, out)
	}

	if _, err := os.Stat(npmCalled); err == nil {
		t.Fatalf("npm should not be invoked when CLAUDE_CODE_REMOTE is unset")
	}
	if _, err := os.Stat(envFile); err == nil {
		t.Fatalf("CLAUDE_ENV_FILE should not be touched when CLAUDE_CODE_REMOTE is unset")
	}
	_ = localBin
}

// TestSetupMise_UnparseableVersionTriggersInstall verifies that when mise is on
// PATH but --version output has no YYYY.M.PATCH token, the hook does not abort
// under set -o pipefail (grep exit 1) and instead falls through to npm install.
func TestSetupMise_UnparseableVersionTriggersInstall(t *testing.T) {
	tmp, home, project, bin, npmDir, localBin := setupTestDirs(t)

	// mise exists but reports a version string the hook cannot parse.
	weirdMise := filepath.Join(bin, "mise")
	if err := os.WriteFile(weirdMise, []byte(fakeMiseScript("not-a-semver-build", "/old/bin")), 0755); err != nil {
		t.Fatal(err)
	}

	npmCalled := filepath.Join(tmp, "npm-called")
	newMise := filepath.Join(localBin, "mise")
	npmBin := filepath.Join(npmDir, "npm")
	if err := os.WriteFile(npmBin, []byte(fakeNpmScript(npmCalled, newMise, localBin)), 0755); err != nil {
		t.Fatal(err)
	}

	envFile := filepath.Join(tmp, "env")
	runHook(t, home, project, envFile, npmDir+":"+bin)

	if _, err := os.Stat(npmCalled); err != nil {
		t.Fatalf("npm was not invoked after unparseable mise --version: %v", err)
	}
}

// TestSetupMise_FindsMiseInHomeLocalBin verifies that a previous install under
// $HOME/.local/bin is detected even when that directory is not already on PATH.
// Cloud sessions may cache the binary on disk without persisting PATH; without
// an early PATH prepend the hook would re-run npm install every SessionStart.
func TestSetupMise_FindsMiseInHomeLocalBin(t *testing.T) {
	tmp, home, project, bin, npmDir, localBin := setupTestDirs(t)

	// Current mise lives only under ~/.local/bin (not on the initial PATH).
	currentMise := filepath.Join(localBin, "mise")
	if err := os.WriteFile(currentMise, []byte(fakeMiseScript("2026.7.7", localBin)), 0755); err != nil {
		t.Fatal(err)
	}

	npmCalled := filepath.Join(tmp, "npm-called")
	npmBin := filepath.Join(npmDir, "npm")
	if err := os.WriteFile(npmBin, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > "+npmCalled+"\n"), 0755); err != nil {
		t.Fatal(err)
	}

	envFile := filepath.Join(tmp, "env")
	// PATH deliberately omits localBin and bin (no other mise).
	runHook(t, home, project, envFile, npmDir)

	if _, err := os.Stat(npmCalled); err == nil {
		t.Fatalf("npm was invoked even though a current mise already exists in $HOME/.local/bin")
	}
	_ = bin
}

// TestSetupMise_InstallsWhenMissing verifies that the hook installs mise via npm
// when there is no mise binary on PATH.
func TestSetupMise_InstallsWhenMissing(t *testing.T) {
	tmp, home, project, bin, npmDir, localBin := setupTestDirs(t)

	npmCalled := filepath.Join(tmp, "npm-called")
	newMise := filepath.Join(localBin, "mise")
	npmBin := filepath.Join(npmDir, "npm")
	if err := os.WriteFile(npmBin, []byte(fakeNpmScript(npmCalled, newMise, localBin)), 0755); err != nil {
		t.Fatal(err)
	}

	envFile := filepath.Join(tmp, "env")
	runHook(t, home, project, envFile, npmDir+":"+bin)

	if _, err := os.Stat(npmCalled); err != nil {
		t.Fatalf("npm was not invoked to install mise: %v", err)
	}
	args, _ := os.ReadFile(npmCalled)
	if !strings.Contains(string(args), "mise@2026.7.7") {
		t.Fatalf("npm install did not target the expected mise version: %s", args)
	}
	if _, err := os.Stat(envFile); err != nil {
		t.Fatalf("expected CLAUDE_ENV_FILE to be written after fresh install: %v", err)
	}
}

// TestSetupMise_TrustFailureContinuesInstall verifies that a non-zero mise trust
// exit does not abort bootstrap: install still runs and CLAUDE_ENV_FILE is written.
func TestSetupMise_TrustFailureContinuesInstall(t *testing.T) {
	tmp, home, project, bin, npmDir, localBin := setupTestDirs(t)

	logPath := filepath.Join(tmp, "mise-log")
	currentMise := filepath.Join(bin, "mise")
	if err := os.WriteFile(currentMise, []byte(fakeMiseScriptRecording(logPath, "2026.7.7", localBin, true, false)), 0755); err != nil {
		t.Fatal(err)
	}

	npmBin := filepath.Join(npmDir, "npm")
	if err := os.WriteFile(npmBin, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}

	envFile := filepath.Join(tmp, "env")
	stdout, stderr, err := runHookSplit(t, home, project, envFile, npmDir+":"+bin)
	if err != nil {
		t.Fatalf("setup-mise.sh failed after trust failure: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("expected mise invocations to be logged: %v", err)
	}
	log := string(logData)
	if !strings.Contains(log, "trust ") && !strings.Contains(log, "trust\n") {
		t.Fatalf("expected trust to be attempted; log:\n%s", log)
	}
	if !hasLogLine(log, "install") {
		t.Fatalf("expected bare install after trust failure; log:\n%s", log)
	}
	if !strings.Contains(string(stderr), "mise trust failed") {
		t.Fatalf("expected stderr warning about trust failure; got: %q", stderr)
	}
	if _, err := os.Stat(envFile); err != nil {
		t.Fatalf("expected CLAUDE_ENV_FILE to be written: %v", err)
	}
	if len(bytes.TrimSpace(stdout)) != 0 {
		t.Fatalf("expected empty stdout; got: %q", stdout)
	}
}

// TestSetupMise_InstallFallbackToGoNode verifies that when bare `mise install`
// fails, the hook retries with `mise install go node` and still writes PATH.
func TestSetupMise_InstallFallbackToGoNode(t *testing.T) {
	tmp, home, project, bin, npmDir, localBin := setupTestDirs(t)

	logPath := filepath.Join(tmp, "mise-log")
	currentMise := filepath.Join(bin, "mise")
	if err := os.WriteFile(currentMise, []byte(fakeMiseScriptRecording(logPath, "2026.7.7", localBin, false, true)), 0755); err != nil {
		t.Fatal(err)
	}

	npmBin := filepath.Join(npmDir, "npm")
	if err := os.WriteFile(npmBin, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}

	envFile := filepath.Join(tmp, "env")
	stdout, stderr, err := runHookSplit(t, home, project, envFile, npmDir+":"+bin)
	if err != nil {
		t.Fatalf("setup-mise.sh failed on install fallback path: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("expected mise invocations to be logged: %v", err)
	}
	log := string(logData)
	if !hasLogLine(log, "install") {
		t.Fatalf("expected bare install attempt; log:\n%s", log)
	}
	if !hasLogLine(log, "install go node") {
		t.Fatalf("expected fallback install go node; log:\n%s", log)
	}
	if !strings.Contains(string(stderr), "installing go and node") {
		t.Fatalf("expected stderr note about fallback; got: %q", stderr)
	}
	if _, err := os.Stat(envFile); err != nil {
		t.Fatalf("expected CLAUDE_ENV_FILE to be written after fallback: %v", err)
	}
	if len(bytes.TrimSpace(stdout)) != 0 {
		t.Fatalf("expected empty stdout; got: %q", stdout)
	}
}

// TestSetupMise_UnsetProjectDirDoesNotAbort verifies that an unset
// CLAUDE_PROJECT_DIR does not abort the hook under `set -u`.
//
// It deliberately does not claim to prove "PWD is used to find mise.toml":
// cmd.Dir already starts the shell in the project directory, so `cd "$PWD"` is a
// no-op and deleting the cd entirely would still pass. The load-bearing
// assertion is that the hook completes and writes CLAUDE_ENV_FILE rather than
// dying on an unbound variable.
func TestSetupMise_UnsetProjectDirDoesNotAbort(t *testing.T) {
	tmp, home, project, bin, npmDir, localBin := setupTestDirs(t)

	currentMise := filepath.Join(bin, "mise")
	if err := os.WriteFile(currentMise, []byte(fakeMiseScript("2026.7.7", localBin)), 0755); err != nil {
		t.Fatal(err)
	}

	npmBin := filepath.Join(npmDir, "npm")
	if err := os.WriteFile(npmBin, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}

	envFile := filepath.Join(tmp, "env")
	cmd := exec.Command("bash", absScript(t))
	cmd.Dir = project
	// Build a clean env without CLAUDE_PROJECT_DIR.
	base := []string{
		"HOME=" + home,
		"CLAUDE_CODE_REMOTE=true",
		"CLAUDE_ENV_FILE=" + envFile,
		"PATH=" + npmDir + ":" + bin + ":/usr/bin:/bin",
	}
	for _, e := range os.Environ() {
		switch {
		case strings.HasPrefix(e, "HOME="),
			strings.HasPrefix(e, "CLAUDE_CODE_REMOTE="),
			strings.HasPrefix(e, "CLAUDE_PROJECT_DIR="),
			strings.HasPrefix(e, "CLAUDE_ENV_FILE="),
			strings.HasPrefix(e, "PATH="):
			continue
		default:
			base = append(base, e)
		}
	}
	cmd.Env = base

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("setup-mise.sh failed without CLAUDE_PROJECT_DIR: %v\n%s", err, out)
	}
	if _, err := os.Stat(envFile); err != nil {
		t.Fatalf("expected CLAUDE_ENV_FILE to be written when using PWD: %v", err)
	}
}

func setupTestDirs(t *testing.T) (tmp, home, project, bin, npmDir, localBin string) {
	tmp = t.TempDir()
	home = filepath.Join(tmp, "home")
	project = filepath.Join(tmp, "project")
	bin = filepath.Join(tmp, "bin")
	npmDir = filepath.Join(tmp, "npm")
	localBin = filepath.Join(home, ".local", "bin")
	for _, d := range []string{home, project, bin, npmDir, localBin} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}

	miseToml := `min_version = "2026.7.7"
[tools]
go = "1.26.5"
node = "24"
`
	if err := os.WriteFile(filepath.Join(project, "mise.toml"), []byte(miseToml), 0644); err != nil {
		t.Fatal(err)
	}
	return
}

func hasLogLine(log, line string) bool {
	for _, l := range strings.Split(log, "\n") {
		if l == line {
			return true
		}
	}
	return false
}

func absScript(t *testing.T) string {
	t.Helper()
	scriptPath := filepath.Join("..", "..", ".claude", "hooks", "setup-mise.sh")
	absScript, err := filepath.Abs(scriptPath)
	if err != nil {
		t.Fatal(err)
	}
	return absScript
}

func fakeMiseScript(version, binPaths string) string {
	return `#!/bin/sh
if [ "$1" = "--version" ]; then echo "` + version + `"; exit 0; fi
if [ "$1" = "trust" ]; then exit 0; fi
if [ "$1" = "install" ]; then exit 0; fi
if [ "$1" = "bin-paths" ]; then echo "` + binPaths + `"; exit 0; fi
exit 0
`
}

// fakeMiseScriptRecording logs each invocation and can fail trust and/or bare install.
// failBareInstall: exit 1 only when `install` is called with no tool args.
func fakeMiseScriptRecording(logPath, version, binPaths string, failTrust, failBareInstall bool) string {
	trustExit := "0"
	if failTrust {
		trustExit = "1"
	}
	bareFail := "false"
	if failBareInstall {
		bareFail = "true"
	}
	return `#!/bin/sh
log=` + logPath + `
echo "$*" >> "$log"
if [ "$1" = "--version" ]; then echo "` + version + `"; exit 0; fi
if [ "$1" = "trust" ]; then exit ` + trustExit + `; fi
if [ "$1" = "install" ]; then
  if [ "` + bareFail + `" = "true" ] && [ "$#" -eq 1 ]; then
    exit 1
  fi
  exit 0
fi
if [ "$1" = "bin-paths" ]; then echo "` + binPaths + `"; exit 0; fi
exit 0
`
}

func fakeMiseScriptWithNoise(version, binPaths string) string {
	return `#!/bin/sh
if [ "$1" = "--version" ]; then
  echo "installing noisy version...
latest version is ` + version + `";
  exit 0;
fi
if [ "$1" = "trust" ]; then echo "trusted mise.toml"; exit 0; fi
if [ "$1" = "install" ]; then echo "installed tools"; exit 0; fi
if [ "$1" = "bin-paths" ]; then echo "` + binPaths + `"; exit 0; fi
exit 0
`
}

func fakeNpmScript(npmCalled, newMise, localBin string) string {
	return `#!/bin/sh
printf '%s\n' "$@" > ` + npmCalled + `
mkdir -p ` + filepath.Dir(newMise) + `
cat > ` + newMise + ` <<'EOF'
#!/bin/sh
if [ "$1" = "--version" ]; then echo "2026.7.7"; exit 0; fi
if [ "$1" = "trust" ]; then exit 0; fi
if [ "$1" = "install" ]; then exit 0; fi
if [ "$1" = "bin-paths" ]; then echo "` + localBin + `"; exit 0; fi
exit 0
EOF
chmod +x ` + newMise + `
`
}

func runHook(t *testing.T, home, project, envFile, path string) []byte {
	t.Helper()
	cmd := exec.Command("bash", absScript(t))
	cmd.Env = append(os.Environ(),
		"HOME="+home,
		"CLAUDE_CODE_REMOTE=true",
		"CLAUDE_PROJECT_DIR="+project,
		"CLAUDE_ENV_FILE="+envFile,
	)
	cmd.Env = append(cmd.Env, "PATH="+path+":/usr/bin:/bin")

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("setup-mise.sh failed: %v\n%s", err, out)
	}
	return out
}

func runHookSplit(t *testing.T, home, project, envFile, path string) (stdout, stderr []byte, err error) {
	t.Helper()
	cmd := exec.Command("bash", absScript(t))
	cmd.Env = append(os.Environ(),
		"HOME="+home,
		"CLAUDE_CODE_REMOTE=true",
		"CLAUDE_PROJECT_DIR="+project,
		"CLAUDE_ENV_FILE="+envFile,
	)
	cmd.Env = append(cmd.Env, "PATH="+path+":/usr/bin:/bin")

	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	err = cmd.Run()
	return outb.Bytes(), errb.Bytes(), err
}
