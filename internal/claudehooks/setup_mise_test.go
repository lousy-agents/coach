package claudehooks

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestCloudEnvSetup_ParityWithMiseToml verifies that the committed cloud
// environment setup script installs the same mise, go, and node versions that
// are pinned in the repository's mise.toml. We cannot auto-apply the cloud
// setup script, but we can guarantee that the copy-paste template does not
// drift.
func TestCloudEnvSetup_ParityWithMiseToml(t *testing.T) {
	tmp := t.TempDir()
	project := filepath.Join(tmp, "project")
	if err := os.MkdirAll(project, 0755); err != nil {
		t.Fatal(err)
	}

	miseToml := `min_version = "2026.7.7"
[settings]
experimental = true
[tools]
go = "1.26.5"
node = "24"
`
	if err := os.WriteFile(filepath.Join(project, "mise.toml"), []byte(miseToml), 0644); err != nil {
		t.Fatal(err)
	}

	setupScript, err := filepath.Abs(filepath.Join("..", "..", ".claude", "cloud-env-setup.sh"))
	if err != nil {
		t.Fatal(err)
	}
	setupBytes, err := os.ReadFile(setupScript)
	if err != nil {
		t.Fatalf("failed to read cloud setup script: %v", err)
	}
	setup := string(setupBytes)

	miseTomlPath, err := filepath.Abs(filepath.Join("..", "..", "mise.toml"))
	if err != nil {
		t.Fatal(err)
	}
	miseBytes, err := os.ReadFile(miseTomlPath)
	if err != nil {
		t.Fatal(err)
	}
	mise := string(miseBytes)

	assertEqual(t, "mise", parseMiseValue("min_version", mise), parseShellVar("mise_version", setup))
	assertEqual(t, "go", parseMiseValue("go", mise), parseShellVar("go_version", setup))
	assertEqual(t, "node", parseMiseValue("node", mise), parseShellVar("node_version", setup))
}

func assertEqual(t *testing.T, name, fromMise, fromScript string) {
	t.Helper()
	if fromMise == "" {
		t.Fatalf("could not extract %s version from mise.toml", name)
	}
	if fromScript == "" {
		t.Fatalf("could not extract %s version from cloud-env-setup.sh", name)
	}
	if fromMise != fromScript {
		t.Fatalf("%s version mismatch: mise.toml=%q cloud-env-setup.sh=%q", name, fromMise, fromScript)
	}
}

func parseMiseValue(key, text string) string {
	if key == "min_version" {
		re := regexp.MustCompile(`^min_version\s*=\s*"([^"]+)"`)
		m := re.FindStringSubmatch(text)
		if len(m) < 2 {
			return ""
		}
		return m[1]
	}
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(key) + `\s*=\s*"([^"]+)"`)
	m := re.FindStringSubmatch(text)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

func parseShellVar(name, text string) string {
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(name) + `="([^"]+)"`)
	m := re.FindStringSubmatch(text)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

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

	// No CLAUDE_CODE_REMOTE set.
	envFile := filepath.Join(tmp, "env")
	cmd := exec.Command("bash", absScript(t))
	cmd.Env = append(os.Environ(),
		"HOME="+home,
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
