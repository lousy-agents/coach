package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var namespaceUnsharePrefix []string

func ensureUnshareAvailable() {
	if _, err := exec.LookPath("unshare"); err != nil {
		Fail(unshareUnavailable)
	}
	if exec.Command("unshare", "-rn", "--", "true").Run() == nil {
		namespaceUnsharePrefix = []string{"unshare", "-rn", "--"}
		return
	}
	if exec.Command("sudo", "unshare", "-n", "--", "true").Run() == nil {
		namespaceUnsharePrefix = []string{"sudo", "unshare", "-n", "--"}
		return
	}
	Fail(unshareUnavailable)
}

func runCoachBaselineUnderUnshare(repo string) (stdout, stderr []byte, exitCode int) {
	prefix := append(append([]string{}, unsharePrefix()...), unshareEnvWrapper(unsharePathEnv(probedNodeExecPath(), os.Getenv("PATH")), writeUnshareGitHome(), os.Getenv("TMPDIR"))...)
	args := append(append([]string{}, prefix...), commandPath, "codesignal", "--baseline", "--project-config", "project.json", "--project-language", "typescript", "--format=json")
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = repo
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	stdout = []byte(outBuf.String())
	stderr = []byte(errBuf.String())
	if err == nil {
		return stdout, stderr, 0
	}
	var exitErr *exec.ExitError
	Expect(errors.As(err, &exitErr)).To(BeTrue(), "expected an ExitError, got: %s (stderr: %s)", err, errBuf.String())
	return stdout, stderr, exitErr.ExitCode()
}

func unsharePrefix() []string {
	return append([]string{}, namespaceUnsharePrefix...)
}

func writeUnshareGitHome() string {
	home, err := os.MkdirTemp("", "coach-unshare-home-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, home)
	Expect(os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("[safe]\n\tdirectory = *\n"), 0o644)).To(Succeed())
	return home
}

func unsharePathEnv(nodeExecPath, ambientPATH string) string {
	dir := filepath.Dir(nodeExecPath)
	if ambientPATH == "" {
		return dir
	}
	return dir + string(os.PathListSeparator) + ambientPATH
}

func unshareEnvWrapper(pathEnv, home, tmpdir string) []string {
	args := []string{"env", "PATH=" + pathEnv, "HOME=" + home}
	if tmpdir != "" {
		args = append(args, "TMPDIR="+tmpdir)
	}
	return args
}
