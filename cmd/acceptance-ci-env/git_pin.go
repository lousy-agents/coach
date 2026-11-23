package main

import (
	"fmt"
	"os/exec"
	"regexp"
)

var gitVersionPattern = regexp.MustCompile(`git version (\d+\.\d+\.\d+)`)

// pinnedGit is the git the tests run under, and how it compares with CI's pin.
type pinnedGit struct {
	path, version, pin, warning string
}

func (g pinnedGit) summary() string {
	return fmt.Sprintf("git %s (%s); CI pins git %s", g.version, g.path, g.pin)
}

// checkGit finds the git to run, from COACH_CI_GIT or PATH, and refuses one
// that is not CI's pinned version unless the caller allowed the mismatch, in
// which case the result carries the line that says the git leg was not covered.
func checkGit(named, pin string, allowMismatch bool) (pinnedGit, error) {
	if pin == "" {
		return pinnedGit{}, fmt.Errorf("COACH_CI_GIT_VERSION is not set; run this through `mise run acceptance-ci-env`, which pins CI's git")
	}
	path, err := gitPath(named)
	if err != nil {
		return pinnedGit{}, err
	}
	out, err := exec.Command(path, "version").Output()
	match := gitVersionPattern.FindStringSubmatch(string(out))
	if err != nil || match == nil {
		return pinnedGit{}, fmt.Errorf("%s did not report a git version (%v): name CI's git %s with COACH_CI_GIT", path, err, pin)
	}
	git := pinnedGit{path: path, version: match[1], pin: pin}
	if git.version == pin {
		return git, nil
	}
	if !allowMismatch {
		return pinnedGit{}, fmt.Errorf("git %s at %s is not CI's pinned git %s; put git %s on PATH or name it with COACH_CI_GIT, or set COACH_CI_GIT_ALLOW_MISMATCH=1 to run only the user and HOME legs", git.version, path, pin, pin)
	}
	git.warning = fmt.Sprintf("WARNING: the git-version leg was NOT covered: these tests ran under git %s, not CI's pinned git %s", git.version, pin)
	return git, nil
}

func gitPath(named string) (string, error) {
	if named == "" {
		path, err := exec.LookPath("git")
		if err != nil {
			return "", fmt.Errorf("no git on PATH: name CI's git with COACH_CI_GIT")
		}
		return path, nil
	}
	path, err := exec.LookPath(named)
	if err != nil {
		return "", fmt.Errorf("COACH_CI_GIT=%s is not an executable git: %v", named, err)
	}
	return path, nil
}
