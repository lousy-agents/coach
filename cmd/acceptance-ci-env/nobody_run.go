package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
)

// nobodysRun prepares a root run: the repository, the unreadable toolchain
// directories on PATH and the module cache are copied where `nobody` can read
// them, and the tests are wrapped in setpriv.
func nobodysRun(base, runDir, gitPath string, withJS bool) (preparedRun, error) {
	uid, gid, err := nobody()
	if err != nil {
		return preparedRun{}, err
	}
	setpriv, err := exec.LookPath("setpriv")
	if err != nil {
		return preparedRun{}, fmt.Errorf("running as root needs setpriv to run the tests as nobody, and none is on PATH")
	}
	repo := filepath.Join(runDir, "repo")
	if err := copyRepository(repo); err != nil {
		return preparedRun{}, err
	}
	var jsNote string
	if withJS {
		if err := copyJSInstalled(repo); err != nil {
			return preparedRun{}, err
		}
		jsNote = "copied the installed " + jsInstalled + " from this tree, as CI's job installs it before its tests"
	}
	path, err := reachablePath(base, os.Getenv("PATH"), filepath.Dir(gitPath), gid)
	if err != nil {
		return preparedRun{}, err
	}
	env, err := nobodysGoCaches(base, uid, gid)
	if err != nil {
		return preparedRun{}, err
	}
	env["PATH"] = path
	if err := chownTree(runDir, uid, gid); err != nil {
		return preparedRun{}, err
	}
	wrap := []string{setpriv, "--reuid=" + strconv.Itoa(uid), "--regid=" + strconv.Itoa(gid), "--clear-groups", "--"}
	return preparedRun{repo: repo, uid: uid, wrap: wrap, env: env, jsNote: jsNote}, nil
}

func nobody() (uid, gid int, err error) {
	account, err := user.Lookup("nobody")
	if err != nil {
		return 0, 0, fmt.Errorf("running as root needs a non-root user to run the tests as, and there is no `nobody` user: %v", err)
	}
	uid, _ = strconv.Atoi(account.Uid)
	gid, _ = strconv.Atoi(account.Gid)
	return uid, gid, nil
}
