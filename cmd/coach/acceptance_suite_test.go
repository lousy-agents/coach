package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var commitEnv = append(os.Environ(),
	"GIT_AUTHOR_NAME=coach-acceptance",
	"GIT_AUTHOR_EMAIL=coach-acceptance@example.com",
	"GIT_COMMITTER_NAME=coach-acceptance",
	"GIT_COMMITTER_EMAIL=coach-acceptance@example.com",
)

var commandPath string

func TestCoachAcceptance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "coach command acceptance suite")
}

var _ = BeforeSuite(func() {
	directory, err := os.MkdirTemp("", "coach-acceptance-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, directory)
	commandPath = filepath.Join(directory, "coach")
	build := exec.Command("go", "build", "-o", commandPath, ".")
	output, err := build.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "building the command: %s", output)
})

// runCoachBinary runs one coach invocation and reports its streams and exit
// code. A failure that is not an *exec.ExitError fails the spec rather than
// being reported as an exit code the command never produced. A nil env
// inherits this process's environment.
func runCoachBinary(binary, workingDir string, env []string, args ...string) (stdout, stderr []byte, exitCode int) {
	command := exec.Command(binary, args...)
	command.Dir = workingDir
	command.Env = env
	var outBuf, errBuf bytes.Buffer
	command.Stdout = &outBuf
	command.Stderr = &errBuf

	err := command.Run()
	if err == nil {
		return outBuf.Bytes(), errBuf.Bytes(), 0
	}
	var exitErr *exec.ExitError
	Expect(errors.As(err, &exitErr)).To(BeTrue(), "expected an ExitError, got: %s (stderr: %s)", err, errBuf.String())
	return outBuf.Bytes(), errBuf.Bytes(), exitErr.ExitCode()
}

// stubToolchainEnv is the scrubbed environment a spec gives coach so it
// resolves node and mise from its own stub directory rather than the host.
func stubToolchainEnv(path string) []string {
	return []string{"PATH=" + path, "HOME=" + os.Getenv("HOME")}
}

func newTempGitRepo() string {
	directory, err := os.MkdirTemp("", "coach-acceptance-repo-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, directory)

	initCmd := exec.Command("git", "init")
	initCmd.Dir = directory
	output, err := initCmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "git init: %s", output)

	return directory
}

// commitFile writes name with contents into repo, commits it, and returns
// the resulting commit's full SHA.
func commitFile(repo, name, contents string) string {
	Expect(os.MkdirAll(filepath.Dir(filepath.Join(repo, name)), 0o755)).To(Succeed())
	err := os.WriteFile(filepath.Join(repo, name), []byte(contents), 0o644)
	Expect(err).NotTo(HaveOccurred())

	addCmd := exec.Command("git", "add", name)
	addCmd.Dir = repo
	output, err := addCmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "git add: %s", output)

	commitCmd := exec.Command("git", "commit", "-m", "commit "+name)
	commitCmd.Dir = repo
	commitCmd.Env = commitEnv
	output, err = commitCmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "git commit: %s", output)

	revCmd := exec.Command("git", "rev-parse", "HEAD")
	revCmd.Dir = repo
	output, err = revCmd.Output()
	Expect(err).NotTo(HaveOccurred())

	return strings.TrimSpace(string(output))
}

// renameFile renames from to to in repo via `git mv` and commits the
// rename, returning the resulting commit's full SHA.
func renameFile(repo, from, to string) string {
	mvCmd := exec.Command("git", "mv", from, to)
	mvCmd.Dir = repo
	output, err := mvCmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "git mv: %s", output)

	commitCmd := exec.Command("git", "commit", "-m", "rename "+from+" to "+to)
	commitCmd.Dir = repo
	commitCmd.Env = commitEnv
	output, err = commitCmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "git commit: %s", output)

	revCmd := exec.Command("git", "rev-parse", "HEAD")
	revCmd.Dir = repo
	output, err = revCmd.Output()
	Expect(err).NotTo(HaveOccurred())

	return strings.TrimSpace(string(output))
}
