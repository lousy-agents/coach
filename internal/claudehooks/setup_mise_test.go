package claudehooks

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestSetupMise_UpgradesStaleVersion verifies that the SessionStart hook
// installs the mise version pinned in mise.toml when an older mise binary is
// already on PATH. This is an acceptance test for .claude/hooks/setup-mise.sh.
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

func fakeMiseScript(version, binPaths string) string {
	return `#!/bin/sh
if [ "$1" = "--version" ]; then echo "` + version + `"; exit 0; fi
if [ "$1" = "trust" ]; then exit 0; fi
if [ "$1" = "install" ]; then exit 0; fi
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
	scriptPath := filepath.Join("..", "..", ".claude", "hooks", "setup-mise.sh")
	absScript, err := filepath.Abs(scriptPath)
	if err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("bash", absScript)
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
