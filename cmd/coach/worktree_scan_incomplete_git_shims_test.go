//go:build unix

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func envWithGitShim(script string) []string {
	directory := GinkgoT().TempDir()
	ExpectWithOffset(2, os.WriteFile(filepath.Join(directory, "git"), []byte(script), 0o755)).To(Succeed())
	return append(os.Environ(), "PATH="+directory+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func realGitPath() string {
	realGit, err := exec.LookPath("git")
	ExpectWithOffset(2, err).NotTo(HaveOccurred())
	return realGit
}

// envWithGitBefore244 runs the git on PATH as a git that reports 2.43.0 and
// ignores the instruction not to fetch.
func envWithGitBefore244() []string {
	script := fmt.Sprintf("#!/bin/sh\nfor last; do :; done\n[ \"$last\" = version ] && { echo 'git version 2.43.0'; exit 0; }\nunset GIT_NO_LAZY_FETCH\nexec %q \"$@\"\n", realGitPath())
	return envWithGitShim(script)
}

// envWithStatusWarnings runs the git on PATH, and has the call that reads the
// status print each line on stderr first.
func envWithStatusWarnings(lines ...string) []string {
	var script strings.Builder
	script.WriteString("#!/bin/sh\nfor argument in \"$@\"; do\n  if [ \"$argument\" = status ]; then\n")
	for _, line := range lines {
		fmt.Fprintf(&script, "    printf '%%s\\n' %q >&2\n", line)
	}
	fmt.Fprintf(&script, "    break\n  fi\ndone\nexec %q \"$@\"\n", realGitPath())
	return envWithGitShim(script.String())
}
