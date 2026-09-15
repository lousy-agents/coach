//go:build linux

package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/sys/unix"
)

// openPTYPair opens a real Linux pseudo-terminal pair via /dev/ptmx,
// duplicating internal/codesignalcli/controlling_terminal_pty_linux_test.go's
// openPTYSlave rather than importing it: it is a test-only fixture in a
// different package, and pty allocation is a handful of ioctls, not shared
// production logic. It returns both ends: master is written to by the spec
// to script the child's interactive answers, slave becomes the child's
// controlling terminal. Every failure calls Skip rather than Fail, so a
// sandbox without pty support produces a loud, named skip in the suite's
// own output -- never a silent pass that would prove nothing about
// AC-POL-8's TTY-gated branch.
func openPTYPair() (master, slave *os.File) {
	m, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		Skip(fmt.Sprintf("open /dev/ptmx: %v (no pty support in this sandbox; AC-POL-8's TTY-gated branch is unproven here)", err))
	}
	if err := unix.IoctlSetPointerInt(int(m.Fd()), unix.TIOCSPTLCK, 0); err != nil {
		m.Close()
		Skip(fmt.Sprintf("TIOCSPTLCK: %v (AC-POL-8's TTY-gated branch is unproven here)", err))
	}
	n, err := unix.IoctlGetInt(int(m.Fd()), unix.TIOCGPTN)
	if err != nil {
		m.Close()
		Skip(fmt.Sprintf("TIOCGPTN: %v (AC-POL-8's TTY-gated branch is unproven here)", err))
	}
	slavePath := fmt.Sprintf("/dev/pts/%d", n)
	s, err := os.OpenFile(slavePath, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		m.Close()
		Skip(fmt.Sprintf("open %s: %v (AC-POL-8's TTY-gated branch is unproven here)", slavePath, err))
	}
	return m, s
}

// runCoachBinaryWithControllingTerminal runs binary with its stdin attached
// to a real controlling terminal (a pty slave) instead of a pipe, so
// codesignalcli.HasControllingTerminal(os.Stdin) is genuinely true inside
// the child -- the only way to exercise AC-POL-8's guided-authoring branch
// from outside the process. stdinScript is written to the pty master before
// the child starts reading; the kernel line discipline buffers it, so exact
// interleaving with the child's own startup does not matter. Stdout/stderr
// stay ordinary pipes: HasControllingTerminal only ever inspects stdin.
func runCoachBinaryWithControllingTerminal(binary, workingDir string, env []string, stdinScript string, args ...string) (stdout, stderr []byte, exitCode int) {
	master, slave := openPTYPair()
	defer master.Close()

	command := exec.Command(binary, args...)
	command.Dir = workingDir
	command.Env = env
	command.Stdin = slave
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true}
	var outBuf, errBuf bytes.Buffer
	command.Stdout = &outBuf
	command.Stderr = &errBuf

	_, writeErr := master.WriteString(stdinScript)
	Expect(writeErr).NotTo(HaveOccurred(), "writing scripted stdin to the pty master")

	startErr := command.Start()
	Expect(slave.Close()).To(Succeed())
	Expect(startErr).NotTo(HaveOccurred(), "starting %s", binary)

	err := command.Wait()
	if err == nil {
		return outBuf.Bytes(), errBuf.Bytes(), 0
	}
	var exitErr *exec.ExitError
	Expect(errors.As(err, &exitErr)).To(BeTrue(), "expected an ExitError, got: %s (stderr: %s)", err, errBuf.String())
	return outBuf.Bytes(), errBuf.Bytes(), exitErr.ExitCode()
}

// D3 already covers the no-controlling-terminal side of the same policy gap
// (project_ts_scan_preflight_acceptance_test.go). These specs cover the
// other side of AC-SET-9's own branch point: a controlling terminal IS
// available, so the scan enters guided policy authoring
// (--suggest-project-config --project-language typescript's own flow)
// instead of only printing a remediation line, and AC-POL-8 governs what
// happens next in that same invocation.
var _ = Describe("coach codesignal (real scan): guided policy authoring on a controlling terminal (AC-POL-8)", func() {
	When("a --baseline scan's explicit --project-config names a policy that was never committed, a controlling terminal is available, and the guided session is approved", func() {
		It("creates the policy candidate but does not analyze it in this invocation: exits 2, emits no CodeSignal report, and instructs the customer to review, commit, and rerun", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")

			path := pathWithStubNode("v24.9.9")
			answers := "1\n\n\n\napprove\n"

			stdout, stderr, exitCode := runCoachBinaryWithControllingTerminal(commandPath, repo, stubToolchainEnv(path), answers,
				"codesignal", "--baseline", "--project-config", "project.json", "--project-language", "typescript", "--format=json")

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(string(stdout)).To(ContainSubstring(`"schema_version"`), "the approved candidate document is emitted, since --output was never set for this invocation")
			Expect(string(stdout)).NotTo(SatisfyAny(ContainSubstring(`"findings"`), ContainSubstring(`"diagnostics"`)), "no CodeSignal report shape may appear on stdout: AC-POL-8 forbids analyzing the candidate in this invocation")
			Expect(string(stderr)).To(ContainSubstring("review it, commit it"), "the customer must be instructed to review and commit the candidate before it is used, stderr: %s", stderr)
			Expect(string(stderr)).To(ContainSubstring("rerun"), "the customer must be instructed to rerun once the candidate is committed, stderr: %s", stderr)
		})
	})

	When("a --baseline scan's explicit --project-config names a policy that was never committed, a controlling terminal is available, and the guided session is declined", func() {
		It("does not create a policy candidate, still emits no CodeSignal report, and does not print the create-and-rerun instruction", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")

			path := pathWithStubNode("v24.9.9")
			answers := "1\n\n\n\nno\n"

			stdout, stderr, exitCode := runCoachBinaryWithControllingTerminal(commandPath, repo, stubToolchainEnv(path), answers,
				"codesignal", "--baseline", "--project-config", "project.json", "--project-language", "typescript", "--format=json")

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty(), "declining approval must never emit a candidate or a report; stdout: %s", stdout)
			Expect(string(stderr)).To(ContainSubstring("authoring was cancelled or not approved"), "stderr: %s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("review it, commit it"), "no candidate was created, so the review/commit/rerun instruction must not appear; stderr: %s", stderr)
		})
	})
})
