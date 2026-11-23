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
	jsNote     string
	uid        int
	env        []string
	wrap       []string
	cleanup    func()
}

func prepareWorkspace(gitPath string, withJS bool) (space workspace, err error) {
	base, err := ownedBase()
	if err != nil {
		return workspace{}, err
	}
	runDir, err := os.MkdirTemp(base, "run-*")
	if err != nil {
		return workspace{}, err
	}
	space = workspace{home: filepath.Join(runDir, "home"), uid: os.Geteuid(), cleanup: func() { _ = os.RemoveAll(runDir) }}
	defer func() {
		if err != nil {
			space.cleanup()
		}
	}()
	overrides, err := isolatedGitAndHome(runDir)
	if err != nil {
		return workspace{}, err
	}
	overrides["PATH"] = filepath.Dir(gitPath) + string(filepath.ListSeparator) + os.Getenv("PATH")
	if space.uid == 0 {
		err = space.dropToNobody(base, runDir, gitPath, overrides, withJS)
	} else {
		space.repo, err = os.Getwd()
		if err == nil {
			err = keepCallersGoCaches(overrides)
		}
		if withJS {
			space.jsNote = "uses js/semantics' built outputs in place, as CI's job builds them before its tests"
		}
	}
	if err != nil {
		return workspace{}, err
	}
	space.env = withOverrides(withoutInheritedGitConfig(os.Environ()), overrides)
	return space, nil
}

// isolatedGitAndHome makes the empty HOME, the empty user-level git config and
// the temporary directory the tests run with, all inside runDir.
func isolatedGitAndHome(runDir string) (map[string]string, error) {
	dirs := map[string]string{"HOME": "home", "XDG_CONFIG_HOME": "xdg", "TMPDIR": "tmp"}
	overrides := map[string]string{}
	for name, dir := range dirs {
		path := filepath.Join(runDir, dir)
		if err := os.MkdirAll(path, 0o755); err != nil {
			return nil, err
		}
		overrides[name] = path
	}
	emptyConfig := filepath.Join(runDir, "gitconfig")
	if err := os.WriteFile(emptyConfig, nil, 0o644); err != nil {
		return nil, err
	}
	overrides["GIT_CONFIG_GLOBAL"] = emptyConfig
	return overrides, nil
}

// dropToNobody prepares a root run: the repository, the unreadable toolchain
// directories on PATH and the module cache are copied where `nobody` can read
// them, and the tests are wrapped in setpriv.
func (w *workspace) dropToNobody(base, runDir, gitPath string, overrides map[string]string, withJS bool) error {
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
	if withJS {
		copied, err := copyJSOutputs(w.repo)
		if err != nil {
			return err
		}
		w.jsNote = fmt.Sprintf("copied js/semantics' built outputs %v from this tree, as CI's job builds them before its tests", copied)
	}
	path, err := reachablePath(base, os.Getenv("PATH"), filepath.Dir(gitPath), gid)
	if err != nil {
		return err
	}
	overrides["PATH"] = path
	if err := useNobodysGoCaches(base, overrides, uid, gid); err != nil {
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
