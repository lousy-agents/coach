//go:build linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"

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

// controllingTerminalCommandTimeout bounds runCoachBinaryWithControllingTerminal's
// child. Without a deadline, a prompt sequence the caller's stdinScript does
// not fully answer leaves the child blocked reading the still-open pty
// forever, and the failure only ever surfaces as the whole package's 10-minute
// test timeout rather than a named spec failure.
const controllingTerminalCommandTimeout = 15 * time.Second

// runCoachBinaryWithControllingTerminal runs binary with its stdin attached
// to a real controlling terminal (a pty slave) instead of a pipe, so
// codesignalcli.HasControllingTerminal(os.Stdin) is genuinely true inside
// the child -- the only way to exercise AC-POL-8's guided-authoring branch
// from outside the process. stdinScript is written to the pty master before
// the child starts reading; the kernel line discipline buffers it, so exact
// interleaving with the child's own startup does not matter. Stdout/stderr
// stay ordinary pipes: HasControllingTerminal only ever inspects stdin.
// terminalTranscript is the master's own read side (the child's echo and any
// of its output the line discipline reflects back); draining it concurrently
// with Wait keeps the finite canonical-mode echo queue from ever
// back-pressuring the child, and gives a future spec (#330 Task 5, which
// answers interleaved output rather than pre-scripting every answer) a
// transcript to assert on.
func runCoachBinaryWithControllingTerminal(binary, workingDir string, env []string, stdinScript string, args ...string) (stdout, stderr, terminalTranscript []byte, exitCode int) {
	master, slave := openPTYPair()
	defer master.Close()

	ctx, cancel := context.WithTimeout(context.Background(), controllingTerminalCommandTimeout)
	defer cancel()

	command := exec.CommandContext(ctx, binary, args...)
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

	var transcript bytes.Buffer
	drained := make(chan struct{})
	go func() {
		io.Copy(&transcript, master)
		close(drained)
	}()

	waitErr := command.Wait()
	<-drained

	Expect(ctx.Err()).NotTo(Equal(context.DeadlineExceeded), "coach did not exit within %s; it is likely blocked reading an unanswered prompt on the pty (stdout: %s, stderr: %s, terminal: %s)", controllingTerminalCommandTimeout, outBuf.String(), errBuf.String(), transcript.String())

	if waitErr == nil {
		return outBuf.Bytes(), errBuf.Bytes(), transcript.Bytes(), 0
	}
	var exitErr *exec.ExitError
	Expect(errors.As(waitErr, &exitErr)).To(BeTrue(), "expected an ExitError, got: %s (stderr: %s)", waitErr, errBuf.String())
	return outBuf.Bytes(), errBuf.Bytes(), transcript.Bytes(), exitErr.ExitCode()
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
		DescribeTable("creates the policy candidate but does not analyze it in this invocation: exits 2, emits no CodeSignal report, and instructs the customer to review, commit, and rerun, regardless of --format (AC-9)",
			func(formatArg string) {
				repo := newTempGitRepo()
				commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
				commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")

				path := pathWithStubNode("v24.9.9")
				answers := "1\n\n\n\napprove\n"

				stdout, stderr, _, exitCode := runCoachBinaryWithControllingTerminal(commandPath, repo, stubToolchainEnv(path), answers,
					"codesignal", "--baseline", "--project-config", "project.json", "--project-language", "typescript", formatArg)

				Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)

				var candidate map[string]any
				Expect(json.Unmarshal(stdout, &candidate)).To(Succeed(), "the approved candidate document must be emitted as parseable JSON regardless of --format, since --output was never set for this invocation; stdout: %s", stdout)
				Expect(candidate).To(HaveKeyWithValue("schema_version", "1"))
				Expect(candidate).To(HaveKey("roots"), "the emitted document must be the project-config candidate, not a CodeSignal report")
				Expect(candidate).NotTo(HaveKey("scope"), "a CodeSignal report's top-level scope/signals/coverage keys must never appear: AC-POL-8 forbids analyzing the candidate in this invocation")
				Expect(candidate).NotTo(HaveKey("signals"))
				Expect(candidate).NotTo(HaveKey("coverage"))
				Expect(string(stdout)).NotTo(ContainSubstring("Coverage:\n"), "the text-format CodeSignal report renderer's own section marker must never appear on this path")

				Expect(string(stderr)).To(ContainSubstring("review it, commit it"), "the customer must be instructed to review and commit the candidate before it is used, stderr: %s", stderr)
				Expect(string(stderr)).To(ContainSubstring("rerun"), "the customer must be instructed to rerun once the candidate is committed, stderr: %s", stderr)
			},
			Entry("--format=json", "--format=json"),
			Entry("--format=text", "--format=text"),
		)
	})

	When("a --baseline scan's explicit --project-config names a policy that was never committed, a controlling terminal is available, and the guided session is declined", func() {
		It("does not create a policy candidate, still emits no CodeSignal report, and does not print the create-and-rerun instruction", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")

			path := pathWithStubNode("v24.9.9")
			answers := "1\n\n\n\nno\n"

			stdout, stderr, _, exitCode := runCoachBinaryWithControllingTerminal(commandPath, repo, stubToolchainEnv(path), answers,
				"codesignal", "--baseline", "--project-config", "project.json", "--project-language", "typescript", "--format=json")

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty(), "declining approval must never emit a candidate or a report; stdout: %s", stdout)
			Expect(string(stderr)).To(ContainSubstring("authoring was cancelled or not approved"), "stderr: %s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("review it, commit it"), "no candidate was created, so the review/commit/rerun instruction must not appear; stderr: %s", stderr)
		})
	})
})

// scanShouldAuthorProjectConfig must trigger only for AC-POL-8's own gap: a
// TypeScript policy that was never committed at all. A *ProjectConfigError
// can also report a usage error (an unusable --project-config path) or a
// file that exists uncommitted in the worktree, and neither is "no policy
// was ever authored" -- entering guided authoring for either would silently
// discard the pre-existing, more specific diagnostic these two regressions
// pin, and (for the worktree case) offer to author a second, redundant
// candidate instead of the customer's actual fix (`git add`).
var _ = Describe("coach codesignal (real scan): guided policy authoring guard is scoped to an absent, never-committed policy (AC-POL-8)", func() {
	When("a --baseline scan's --project-config value fails path-shape validation, and a controlling terminal is available", func() {
		It("prints the pre-existing usage diagnostic unchanged and never enters guided authoring", func() {
			repo := newTempGitRepo()
			commitFile(repo, "README.md", "seed\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, _, exitCode := runCoachBinaryWithControllingTerminal(commandPath, repo, stubToolchainEnv(path), "",
				"codesignal", "--baseline", "--project-config", "/etc/passwd", "--project-language", "typescript", "--format=json")

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty(), "a usage error must never enter guided authoring or emit a candidate document; stdout: %s", stdout)
			Expect(string(stderr)).To(ContainSubstring("path must be a non-empty repository-relative path"), "the pre-existing path-validation message must be preserved unchanged; stderr: %s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("Discovered TypeScript roots"), "an absolute --project-config path must never open a guided authoring session; stderr: %s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("review it, commit it"), "no candidate can have been created for a usage error; stderr: %s", stderr)
		})
	})

	When("a --baseline scan's --project-config names a file that exists uncommitted in the worktree, and a controlling terminal is available", func() {
		It("prints the pre-existing uncommitted-in-worktree diagnostic unchanged and never enters guided authoring", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
			writeWorktreeFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, _, exitCode := runCoachBinaryWithControllingTerminal(commandPath, repo, stubToolchainEnv(path), "",
				"codesignal", "--baseline", "--project-config", "project.json", "--project-language", "typescript", "--format=json")

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty(), "an uncommitted-in-worktree gap must never enter guided authoring or emit a second candidate; stdout: %s", stdout)
			Expect(string(stderr)).To(ContainSubstring("exists in the worktree but is not committed"), "the pre-existing uncommitted-in-worktree message must be preserved unchanged; stderr: %s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("Discovered TypeScript roots"), "a file that already exists uncommitted must never open a guided authoring session; the fix is `git add`, not authoring a second candidate; stderr: %s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("review it, commit it"), "no candidate can have been created for this gap; stderr: %s", stderr)
		})
	})
})

// These two specs are pins, not red-then-green regressions: both exclusions
// already behaved correctly on a controlling terminal before the guard
// above was narrowed (confirmed by hand against the built binary). Without
// them, nothing stops a later edit to scanShouldAuthorProjectConfig from
// widening it back over these two cases the same way it did over the two
// regressions pinned above.
var _ = Describe("coach codesignal (real scan): guided policy authoring guard leaves other gaps on a controlling terminal untouched (AC-17)", func() {
	When("a --baseline scan resolves no TypeScript compiler (not a project-config gap), and a controlling terminal is available", func() {
		It("keeps the existing --check-project remediation as the only stderr line and never enters guided authoring", func() {
			repo := newTempGitRepo()
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, _, exitCode := runCoachBinaryWithControllingTerminal(commandPath, repo, stubToolchainEnv(path), "",
				"codesignal", "--baseline", "--project-config", "project.json", "--project-language", "typescript", "--format=json")

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty(), "stdout must stay reserved for the final report; none was produced")

			lines := stderrLines(stderr)
			Expect(lines).To(HaveLen(1), "a controlling terminal withholds AC-SET-9's appended remediation line entirely (Task 7 owns that offer); stderr: %s", stderr)
			Expect(lines[0]).To(Equal("typescript_compiler_missing: run coach codesignal --baseline --check-project --project-language typescript --project-config project.json"), "the existing remediation line must not change; a CompilerUnresolvedError must never reach the TypeScript-specific guided authoring session")
		})
	})

	When("a --baseline scan's --project-config gap is evaluated with a non-TypeScript --project-language, and a controlling terminal is available", func() {
		It("keeps the existing project_config_invalid message unchanged and never enters the TypeScript-specific guided authoring session", func() {
			repo := newTempGitRepo()
			commitFile(repo, "a.go", "package a\n\nfunc A() {}\n")

			stdout, stderr, _, exitCode := runCoachBinaryWithControllingTerminal(commandPath, repo, nil, "",
				"codesignal", "--baseline", "--project-config", "project.json", "--project-language", "go", "--format=json")

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty())

			lines := stderrLines(stderr)
			Expect(lines).To(HaveLen(1), "a controlling terminal withholds AC-SET-9's appended remediation line entirely; stderr: %s", stderr)
			Expect(lines[0]).To(ContainSubstring("project_config_invalid"), "the existing class-2 message must still be reported")
			Expect(string(stderr)).NotTo(ContainSubstring("Discovered TypeScript roots"), "the guided authoring session below is TypeScript-specific and must never run for --project-language go")
		})
	})
})
