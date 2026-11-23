package main

import (
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
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
	cleanup := func() { _ = os.RemoveAll(runDir) }
	space = workspace{home: filepath.Join(runDir, "home"), uid: os.Geteuid(), cleanup: cleanup}
	defer func() {
		if err != nil {
			cleanup()
		}
	}()
	isolation, err := isolatedGitAndHome(runDir)
	if err != nil {
		return workspace{}, err
	}
	run, err := runAs(space.uid, base, runDir, gitPath, withJS)
	if err != nil {
		return workspace{}, err
	}
	space.repo, space.uid, space.wrap, space.jsNote = run.repo, run.uid, run.wrap, run.jsNote
	overrides := maps.Clone(isolation)
	maps.Copy(overrides, run.env)
	space.env = withOverrides(withoutInheritedGitConfig(os.Environ()), overrides)
	return space, nil
}

// runAs is the run of a non-root caller, in place, or of a root caller, as
// nobody over copies.
func runAs(uid int, base, runDir, gitPath string, withJS bool) (preparedRun, error) {
	if uid == 0 {
		return nobodysRun(base, runDir, gitPath, withJS)
	}
	return callersRun(gitPath, withJS)
}

// preparedRun is what one kind of run contributes: where the tests run, as
// whom, the environment it sets, and what it did with the installed JS
// outputs.
type preparedRun struct {
	repo   string
	uid    int
	wrap   []string
	env    map[string]string
	jsNote string
}

// callersRun runs the tests in place as the caller, with the checked git
// first on PATH and the caller's Go caches named outside the empty HOME.
func callersRun(gitPath string, withJS bool) (preparedRun, error) {
	repo, err := os.Getwd()
	if err != nil {
		return preparedRun{}, err
	}
	env, err := callersGoCaches()
	if err != nil {
		return preparedRun{}, err
	}
	env["PATH"] = filepath.Dir(gitPath) + string(filepath.ListSeparator) + os.Getenv("PATH")
	run := preparedRun{repo: repo, uid: os.Geteuid(), env: env}
	if withJS {
		run.jsNote = "uses the installed " + jsInstalled + " in place, as CI's job installs it before its tests"
	}
	return run, nil
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
