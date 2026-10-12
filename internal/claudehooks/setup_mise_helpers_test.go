package claudehooks

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

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

func absScript(t *testing.T) string {
	t.Helper()
	scriptPath := filepath.Join("..", "..", ".claude", "hooks", "setup-mise.sh")
	absScript, err := filepath.Abs(scriptPath)
	if err != nil {
		t.Fatal(err)
	}
	return absScript
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

func hasLogLine(log, line string) bool {
	for _, l := range strings.Split(log, "\n") {
		if l == line {
			return true
		}
	}
	return false
}
