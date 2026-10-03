package claudehooks

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

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

func hasLogLine(log, line string) bool {
	for _, l := range strings.Split(log, "\n") {
		if l == line {
			return true
		}
	}
	return false
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
