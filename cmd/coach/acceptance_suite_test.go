package main

import (
	"bytes"
	"errors"
	"fmt"
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

// coachNameStatusFlags are the rename/copy detection options SelectChangedFiles
// passes to `git diff --name-status`. Specs that claim an R/C outcome must
// observe this exact invocation, not git's defaults or a local diff.renames
// config.
var coachNameStatusFlags = []string{"--find-renames", "--find-copies-harder"}

const hiddenInputMutationFn = "\nfunc Update(input *int) {\n\t*input = 1\n}\n"

const hiddenInputMutationGo = "package a\n" + hiddenInputMutationFn

// paddedGoPackage returns a Go file large enough that git's default 50%
// rename/copy threshold still reports R/C after a modest HEAD-side append.
func paddedGoPackage(pkg string, funcs int, extra string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "package %s\n\n", pkg)
	for i := 0; i < funcs; i++ {
		fmt.Fprintf(&b, "func Pad%d() int { return %d }\n", i, i)
	}
	b.WriteString(extra)
	return b.String()
}

// coachNameStatus runs `git diff --name-status` with the same rename/copy
// flags coach passes, so a spec can pin R/C before asserting report behavior.
func coachNameStatus(repo, base string) string {
	args := append([]string{"diff", "--name-status"}, coachNameStatusFlags...)
	args = append(args, base, "HEAD")
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	output, err := cmd.Output()
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "git diff --name-status with coach flags: %s", output)
	return string(output)
}

// expectCoachStatusPrefix asserts that coach's name-status flags report
// newPath with a status whose first byte is prefix (R or C). A fixture that
// lands below git's similarity threshold is D+A today and would false-green
// the rename/copy analysis specs.
func expectCoachStatusPrefix(repo, base, newPath, prefix string) {
	status := coachNameStatus(repo, base)
	found := ""
	for _, line := range strings.Split(strings.TrimSpace(status), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) == 0 {
			continue
		}
		path := fields[len(fields)-1]
		if path == newPath {
			found = fields[0]
			break
		}
	}
	ExpectWithOffset(1, found).NotTo(BeEmpty(), "coach name-status missing %q:\n%s", newPath, status)
	ExpectWithOffset(1, strings.HasPrefix(found, prefix)).To(BeTrue(),
		"coach name-status for %q is %q, want prefix %q (threshold trap):\n%s", newPath, found, prefix, status)
}

// renameAndWrite stages `git mv` then overwrites the new path and commits
// both as one change, so git reports a scored rename rather than R100.
func renameAndWrite(repo, from, to, contents string) string {
	mvCmd := exec.Command("git", "mv", from, to)
	mvCmd.Dir = repo
	output, err := mvCmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "git mv: %s", output)

	Expect(os.WriteFile(filepath.Join(repo, to), []byte(contents), 0o644)).To(Succeed())
	addCmd := exec.Command("git", "add", to)
	addCmd.Dir = repo
	output, err = addCmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "git add: %s", output)

	commitCmd := exec.Command("git", "commit", "-m", "rename "+from+" to "+to+" with edits")
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

// typechangeFileToSymlink replaces path with a symlink to target and commits
// the change so git reports status T (still unanalyzed after R/C selection).
func typechangeFileToSymlink(repo, path, target string) string {
	Expect(os.Remove(filepath.Join(repo, path))).To(Succeed())
	Expect(os.Symlink(target, filepath.Join(repo, path))).To(Succeed())

	addCmd := exec.Command("git", "add", path)
	addCmd.Dir = repo
	output, err := addCmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "git add typechange: %s", output)

	commitCmd := exec.Command("git", "commit", "-m", "typechange "+path+" to symlink")
	commitCmd.Dir = repo
	commitCmd.Env = commitEnv
	output, err = commitCmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "git commit typechange: %s", output)

	revCmd := exec.Command("git", "rev-parse", "HEAD")
	revCmd.Dir = repo
	output, err = revCmd.Output()
	Expect(err).NotTo(HaveOccurred())
	return strings.TrimSpace(string(output))
}
