package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

// workspace is where and as whom the tests run. As a non-root user the tests
// run in place; as root they run as `nobody` over copies that user can read.
type workspace struct {
	repo, home string
	uid        int
	env        []string
	wrap       []string
	cleanup    func()
}

func prepareWorkspace(gitPath string) (workspace, error) {
	base := os.Getenv("COACH_CI_ENV_CACHE")
	if base == "" {
		base = filepath.Join(os.TempDir(), "coach-ci-env-"+strconv.Itoa(os.Geteuid()))
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		return workspace{}, err
	}
	runDir, err := os.MkdirTemp(base, "run-*")
	if err != nil {
		return workspace{}, err
	}
	space := workspace{home: filepath.Join(runDir, "home"), uid: os.Geteuid(), cleanup: func() { _ = os.RemoveAll(runDir) }}
	for _, dir := range []string{space.home, filepath.Join(runDir, "xdg")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return workspace{}, err
		}
	}
	emptyConfig := filepath.Join(runDir, "gitconfig")
	if err := os.WriteFile(emptyConfig, nil, 0o644); err != nil {
		return workspace{}, err
	}
	overrides := map[string]string{
		"HOME":              space.home,
		"GIT_CONFIG_GLOBAL": emptyConfig,
		"XDG_CONFIG_HOME":   filepath.Join(runDir, "xdg"),
	}
	if space.uid == 0 {
		if err := space.dropToNobody(base, runDir, gitPath, overrides); err != nil {
			space.cleanup()
			return workspace{}, err
		}
	} else if space.repo, err = os.Getwd(); err != nil {
		return workspace{}, err
	}
	space.env = withOverrides(os.Environ(), overrides)
	return space, nil
}

// dropToNobody prepares a root run: the repository, the unreadable toolchain
// directories on PATH and the module cache are copied where `nobody` can read
// them, and the tests are wrapped in setpriv.
func (w *workspace) dropToNobody(base, runDir, gitPath string, overrides map[string]string) error {
	uid, gid, err := nobody()
	if err != nil {
		return err
	}
	setpriv, err := exec.LookPath("setpriv")
	if err != nil {
		return fmt.Errorf("running as root needs setpriv to run the tests as nobody, and none is on PATH")
	}
	w.repo = filepath.Join(runDir, "repo")
	if err := copyRepository(w.repo); err != nil {
		return err
	}
	path, err := reachablePath(base, os.Getenv("PATH"), filepath.Dir(gitPath))
	if err != nil {
		return err
	}
	overrides["PATH"] = path
	if err := w.useReachableGoCaches(base, overrides); err != nil {
		return err
	}
	if err := chownTree(runDir, uid, gid); err != nil {
		return err
	}
	w.uid = uid
	w.wrap = []string{setpriv, "--reuid=" + strconv.Itoa(uid), "--regid=" + strconv.Itoa(gid), "--clear-groups", "--"}
	return nil
}

func (w workspace) goTest(args []string, stdout, stderr io.Writer) int {
	argv := append(append(append([]string{}, w.wrap...), "go", "test"), args...)
	command := exec.Command(argv[0], argv[1:]...)
	command.Dir = w.repo
	command.Env = w.env
	command.Stdout, command.Stderr = stdout, stderr
	if err := command.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		fmt.Fprintln(stderr, prefix+err.Error())
		return 2
	}
	return 0
}

// unset, as an override, removes the variable: a GOROOT that names the root
// user's toolchain would send the non-root go to a directory it cannot read.
const unset = "\x00unset"

func withOverrides(environ []string, overrides map[string]string) []string {
	var out []string
	for _, entry := range environ {
		name, _, _ := cutEnv(entry)
		if _, overridden := overrides[name]; !overridden {
			out = append(out, entry)
		}
	}
	for name, value := range overrides {
		if value != unset {
			out = append(out, name+"="+value)
		}
	}
	return out
}

func cutEnv(entry string) (string, string, bool) {
	for i := 0; i < len(entry); i++ {
		if entry[i] == '=' {
			return entry[:i], entry[i+1:], true
		}
	}
	return entry, "", false
}
