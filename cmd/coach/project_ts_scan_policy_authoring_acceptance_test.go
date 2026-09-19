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
	"strings"
	"sync"
	"syscall"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/sys/unix"

	"github.com/lousy-agents/coach/internal/codesignalcli"
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
// back-pressuring the child, and gives a future spec that answers
// interleaved output rather than pre-scripting every answer a transcript to
// assert on.
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

// syncBuffer is a goroutine-safe bytes.Buffer: os/exec reads a command's
// Stdout/Stderr pipes on their own internal goroutines, so a spec polling
// the same buffer from the test goroutine (waitForPrompt, below) needs its
// own synchronization -- a plain bytes.Buffer assigned directly as
// cmd.Stdout/Stderr is not safe for that concurrent read.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

func (s *syncBuffer) Bytes() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]byte(nil), s.buf.Bytes()...)
}

// controllingTerminalSession is runCoachBinaryWithControllingTerminal's
// streaming counterpart. That function writes stdinScript to the pty master
// in full before the child starts and only becomes readable after Wait, so
// nothing can read a prompt and choose the next answer from it -- and its
// pre-write is capped by the terminal's ~4 KiB canonical-mode input queue,
// which a long scripted session (a menu, a preview, then a confirmation)
// can exceed. A controllingTerminalSession instead exposes waitForPrompt
// (block until a substring appears in stderr so far) and writeLine (send
// the next answer once that substring has appeared), so a spec can drive an
// interactive session whose next answer depends on output the child
// produces at runtime. The same ctx deadline governs the whole session and
// every individual waitForPrompt call, so an unanswered prompt still fails
// by a named Gomega assertion rather than hanging until the package's own
// test timeout.
type controllingTerminalSession struct {
	ctx     context.Context
	cancel  context.CancelFunc
	command *exec.Cmd
	master  *os.File

	mu         sync.Mutex
	transcript bytes.Buffer
	drained    chan struct{}

	stdoutBuf *syncBuffer
	stderrBuf *syncBuffer
}

// startCoachBinaryWithControllingTerminal starts binary with its stdin
// attached to a real controlling terminal, exactly like
// runCoachBinaryWithControllingTerminal, but returns control to the spec
// before the child has necessarily produced any output, so the caller can
// interleave waitForPrompt/writeLine calls with the child's own prompts
// instead of pre-scripting every answer. waitForPrompt polls stderr, not
// the pty's own transcript: every interactive prompt this package's CLI
// dispatch prints goes to stderr (a regular pipe, distinct from the
// terminal), and the pty transcript otherwise only carries the line
// discipline's echo of what the spec itself typed.
func startCoachBinaryWithControllingTerminal(binary, workingDir string, env []string, args ...string) *controllingTerminalSession {
	master, slave := openPTYPair()

	ctx, cancel := context.WithTimeout(context.Background(), controllingTerminalCommandTimeout)

	command := exec.CommandContext(ctx, binary, args...)
	command.Dir = workingDir
	command.Env = env
	command.Stdin = slave
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true}
	outBuf, errBuf := &syncBuffer{}, &syncBuffer{}
	command.Stdout = outBuf
	command.Stderr = errBuf

	startErr := command.Start()
	Expect(slave.Close()).To(Succeed())
	Expect(startErr).NotTo(HaveOccurred(), "starting %s", binary)

	session := &controllingTerminalSession{
		ctx: ctx, cancel: cancel, command: command, master: master,
		drained: make(chan struct{}), stdoutBuf: outBuf, stderrBuf: errBuf,
	}
	go func() {
		defer close(session.drained)
		buf := make([]byte, 4096)
		for {
			n, readErr := master.Read(buf)
			if n > 0 {
				session.mu.Lock()
				session.transcript.Write(buf[:n])
				session.mu.Unlock()
			}
			if readErr != nil {
				return
			}
		}
	}()
	return session
}

func (s *controllingTerminalSession) transcriptSoFar() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.transcript.String()
}

// waitForPrompt blocks until substr has appeared anywhere in stderr read so
// far, polling rather than requiring the production prompt text to be
// flushed in any particular chunking. It fails the spec (by name) once the
// session's shared ctx deadline elapses, instead of blocking forever on a
// prompt the scripted answers never satisfy.
func (s *controllingTerminalSession) waitForPrompt(substr string) {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if strings.Contains(s.stderrBuf.String(), substr) {
			return
		}
		select {
		case <-s.ctx.Done():
			Fail(fmt.Sprintf("timed out waiting for prompt %q (stderr so far: %s, transcript so far: %s)", substr, s.stderrBuf.String(), s.transcriptSoFar()))
		case <-ticker.C:
		}
	}
}

// writeLine sends line plus a trailing newline to the child's controlling
// terminal, as if a person had typed it and pressed enter.
func (s *controllingTerminalSession) writeLine(line string) {
	_, err := s.master.WriteString(line + "\n")
	Expect(err).NotTo(HaveOccurred(), "writing %q to the pty master", line)
}

// wait closes out the session -- waiting for the child to exit, draining
// the rest of its terminal output, and checking the shared deadline was
// never exceeded -- and returns stdout/stderr/transcript/exit code in the
// same shape runCoachBinaryWithControllingTerminal returns, so a spec that
// no longer needs to interleave answers can read the result identically.
func (s *controllingTerminalSession) wait() (stdout, stderr, transcript []byte, exitCode int) {
	defer s.cancel()
	waitErr := s.command.Wait()
	<-s.drained
	s.master.Close()

	Expect(s.ctx.Err()).NotTo(Equal(context.DeadlineExceeded), "coach did not exit within %s; it is likely blocked reading an unanswered prompt on the pty (stdout: %s, stderr: %s, terminal: %s)", controllingTerminalCommandTimeout, s.stdoutBuf.String(), s.stderrBuf.String(), s.transcriptSoFar())

	if waitErr == nil {
		return s.stdoutBuf.Bytes(), s.stderrBuf.Bytes(), []byte(s.transcriptSoFar()), 0
	}
	var exitErr *exec.ExitError
	Expect(errors.As(waitErr, &exitErr)).To(BeTrue(), "expected an ExitError, got: %s (stderr: %s)", waitErr, s.stderrBuf.String())
	return s.stdoutBuf.Bytes(), s.stderrBuf.Bytes(), []byte(s.transcriptSoFar()), exitErr.ExitCode()
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
				Expect(string(stderr)).To(ContainSubstring("no file was created"), "a scan never sets --output, so the approved candidate goes to stdout and nothing is written to disk; telling the customer a candidate \"was created\" names an artifact they cannot find, review, or commit. stderr: %s", stderr)
				Expect(string(stderr)).To(ContainSubstring(`"project.json"`), "the instruction must name the path the scan itself required, so the customer knows where to save what they just approved; stderr: %s", stderr)

				// AC-SET-13's "report all gaps" clause must hold on this
				// controlling-terminal branch too, not only on the no-TTY path
				// (project_ts_scan_preflight_acceptance_test.go): this fixture
				// has no installed TypeScript compiler either, so both gaps
				// are real and simultaneous. Reporting only the masking policy
				// failure here would let the customer approve, commit, rerun,
				// and only then discover the compiler is also missing.
				Expect(string(stderr)).To(ContainSubstring("project_config_invalid"), "the masking policy failure must still be reported before guided authoring opens; stderr: %s", stderr)
				Expect(string(stderr)).To(ContainSubstring("typescript_compiler_missing: also failing,"), "AC-SET-13 requires the simultaneously failing compiler gap to be reported too, not just the masking policy failure; stderr: %s", stderr)
				gapIndex := strings.Index(string(stderr), "typescript_compiler_missing: also failing,")
				authoringIndex := strings.Index(string(stderr), "Discovered TypeScript roots")
				Expect(authoringIndex).To(BeNumerically(">", gapIndex), "both gaps must be reported before the interactive guided-authoring session opens, not interleaved with or after it; stderr: %s", stderr)
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

// AC-10 requires no setup mutation before a reviewed, committed policy
// exists. The both-gaps fixture above proves nothing here on its own: it has
// no installable compiler-setup choice at all, so "nothing was installed"
// would hold vacuously even if the code below were removed entirely. This
// fixture adds a genuinely installable mise scope alongside the
// never-committed policy so the compiler check genuinely fails too, and the
// assertions below hold for this repository shape.
//
// This is not ordering coverage, and must not be read as such: a
// never-committed policy always surfaces as a *ProjectConfigError before the
// TypeScript backend ever runs its own compiler check (prepareProjectAnalysis's
// loadProjectConfig short circuit, project.go), so a real scan's error here
// is never simultaneously a *ProjectConfigError and a
// *CompilerUnresolvedErrorWithReadiness. AC-10's guarantee is therefore
// structural, not enforced by which `if` runs first in dispatchScanError
// (main.go) -- swapping that order changes nothing this fixture, or any real
// scan, can ever reach.
var _ = Describe("coach codesignal (real scan): guided policy authoring never mutates compiler setup before a reviewed, committed policy exists (AC-10)", func() {
	When("a --baseline scan's --project-config names a policy that was never committed, the repository also fails its compiler check, and a real installable mise scope is available", func() {
		It("never opens the interactive compiler-setup offer and never runs mise install before guided policy authoring", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
			writeWorktreeFile(repo, "mise.toml", fmt.Sprintf("[tools]\n\"npm:typescript\" = %q\n", codesignalcli.SupportedTypescriptVersions[0]))
			miseDir := writeStatefulStubMiseScript(codesignalcli.SupportedTypescriptVersions[0])
			path := writeStubNodeScript("v24.9.9") + string(os.PathListSeparator) + miseDir + string(os.PathListSeparator) + pathExcludingToolchain()

			answers := "1\n\n\n\napprove\n"
			stdout, stderr, _, exitCode := runCoachBinaryWithControllingTerminal(commandPath, repo, stubToolchainEnv(path), answers,
				"codesignal", "--baseline", "--project-config", "project.json", "--project-language", "typescript", "--format=json")

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("TypeScript compiler setup:"), "no interactive compiler-setup offer must ever open before a reviewed, committed policy exists; stderr: %s", stderr)
			Expect(miseInvocationsIncludeInstall(miseDir)).To(BeFalse(), "no setup mutation must occur before a reviewed, committed policy exists")
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

// The first two specs below also guard the interactive compiler-setup offer
// itself (AC-8, AC-9): a compiler gap with nothing installable must resolve
// to the pre-existing single remediation line without ever opening a
// prompt, and a compiler gap that does offer a real choice must present the
// menu and honor cancellation without invoking anything. The last spec
// guards AppendedRemediationLine's own language gate: only TypeScript has an
// interactive setup offer to own the controlling-terminal case, so a --project-
// language go class-2 config failure must still see the same appended
// remediation a no-controlling-terminal invocation gets (project_contract_
// acceptance_test.go pins that no-TTY shape).
var _ = Describe("coach codesignal (real scan): guided policy authoring guard leaves other gaps on a controlling terminal untouched (AC-17)", func() {
	When("a --baseline scan resolves no TypeScript compiler (not a project-config gap), no package manager and no mise are available to set it up, and a controlling terminal is available", func() {
		It("never enters guided POLICY authoring, never opens the compiler-setup prompt either (there is nothing installable to offer), keeps the existing --check-project remediation as the only stderr line, and exits 2", func() {
			repo := newTempGitRepo()
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, _, exitCode := runCoachBinaryWithControllingTerminal(commandPath, repo, stubToolchainEnv(path), "",
				"codesignal", "--baseline", "--project-config", "project.json", "--project-language", "typescript", "--format=json")

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty(), "stdout must stay reserved for the final report; none was produced")
			Expect(string(stderr)).NotTo(ContainSubstring("Discovered TypeScript roots"), "a CompilerUnresolvedError must never reach the TypeScript-specific guided POLICY authoring session -- that is a different gap entirely")

			lines := stderrLines(stderr)
			Expect(lines).To(HaveLen(2), "a compiler gap with no installable choice must never open the interactive compiler-setup prompt, but must say why nothing was offered; stderr: %s", stderr)
			Expect(lines[0]).To(Equal("typescript_compiler_missing: run coach codesignal --baseline --check-project --project-language typescript --project-config project.json"), "the existing remediation line must not change")
			Expect(lines[1]).To(HavePrefix("coach codesignal: no compiler-setup choice is executable here: "), "AvailableSetupChoices computed a reason for every choice it ruled out; dropping them leaves the customer with a --check-project loop that re-reports the same gap and no way to break it. stderr: %s", stderr)
			Expect(lines[1]).To(ContainSubstring("project_package ("), "the withheld project-package choice and its reason must reach the customer; stderr: %s", stderr)
			Expect(lines[1]).To(ContainSubstring("project_mise ("), "the withheld mise scopes and their reasons must reach the customer; stderr: %s", stderr)
		})
	})

	When("a --baseline scan resolves no TypeScript compiler, a real mise scope is offered to set it up, and the offer is cancelled", func() {
		It("presents the compiler-setup menu, leaves the pre-existing remediation line untouched, reports the cancellation distinctly from guided policy authoring's own, and never invokes mise install", func() {
			repo := noSupportedCompilerRepo()
			writeWorktreeFile(repo, "mise.toml", fmt.Sprintf("[tools]\n\"npm:typescript\" = %q\n", codesignalcli.SupportedTypescriptVersions[0]))
			miseDir := writeStatefulStubMiseScript(codesignalcli.SupportedTypescriptVersions[0])
			path := writeStubNodeScript("v24.9.9") + string(os.PathListSeparator) + miseDir + string(os.PathListSeparator) + pathExcludingToolchain()

			session := startCoachBinaryWithControllingTerminal(commandPath, repo, stubToolchainEnv(path),
				"codesignal", "--baseline", "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			session.waitForPrompt("an unrecognized or blank answer cancels.")
			session.writeLine("cancel")
			stdout, stderr, _, exitCode := session.wait()

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty(), "stdout must stay reserved for the final report; none was produced")
			Expect(string(stderr)).NotTo(ContainSubstring("Discovered TypeScript roots"), "a CompilerUnresolvedError must never reach the TypeScript-specific guided POLICY authoring session -- that is a different gap entirely")
			Expect(string(stderr)).To(ContainSubstring("typescript_compiler_missing: run coach codesignal --baseline --check-project --project-language typescript --project-config project.json"), "the pre-existing D3 remediation line must still be printed once compiler setup is cancelled")
			Expect(string(stderr)).To(ContainSubstring("compiler setup was cancelled"), "cancelling this compiler-setup offer must be reported distinctly from guided policy authoring's own cancellation message")
			Expect(string(stderr)).To(ContainSubstring("this scan cannot resolve a supported TypeScript compiler (typescript_compiler_missing)"), "the gap must be diagnosed before the menu asks the customer to pick an installation; a list of machine identifiers is not an explanation. stderr: %s", stderr)
			Expect(string(stderr)).To(ContainSubstring("resumes this same scan only if that recheck reports no gap; otherwise no report is produced"), "what happens after a confirmed, successful setup must be disclosed before consent, and disclosed accurately: shouldContinueAfterSetup also requires the readiness recheck to come back clean, so promising a resume on success alone overpromises. stderr: %s", stderr)
			Expect(string(stderr)).To(ContainSubstring("- project_mise (installs the version this repository's mise configuration pins"), "selection must not be made from bare machine identifiers: project_mise and global_mise differ in exactly the property a customer needs before choosing. stderr: %s", stderr)
			Expect(miseInvocationsIncludeInstall(miseDir)).To(BeFalse(), "cancelling the offer must never invoke `mise install`")
		})
	})

	When("a --baseline scan's --project-config gap is evaluated with a non-TypeScript --project-language, and a controlling terminal is available", func() {
		It("keeps the existing project_config_invalid message unchanged, still appends AC-SET-9's remediation since go has no interactive setup offer to own the controlling-terminal case, and never enters the TypeScript-specific guided authoring session", func() {
			repo := newTempGitRepo()
			commitFile(repo, "a.go", "package a\n\nfunc A() {}\n")

			stdout, stderr, _, exitCode := runCoachBinaryWithControllingTerminal(commandPath, repo, nil, "",
				"codesignal", "--baseline", "--project-config", "project.json", "--project-language", "go", "--format=json")

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty())

			lines := stderrLines(stderr)
			Expect(lines).To(HaveLen(2), "a controlling terminal must not withhold AC-SET-9's appended remediation for go: no interactive setup offer exists to own that case; stderr: %s", stderr)
			Expect(lines[0]).To(ContainSubstring("project_config_invalid"), "the existing class-2 message must still be reported")
			Expect(lines[1]).To(Equal("coach codesignal --baseline --suggest-project-config"), "go's appended remediation must be the same command a no-controlling-terminal invocation prints")
			Expect(string(stderr)).NotTo(ContainSubstring("Discovered TypeScript roots"), "the guided authoring session below is TypeScript-specific and must never run for --project-language go")
		})
	})
})

// Exercising this requires a genuine controlling terminal on os.Stdin, since
// scanShouldAuthorProjectConfig reads os.Stdin directly and
// codesignalcli.HasControllingTerminal has no fake-injection mode by design
// (controlling_terminal.go) -- a pipe or regular file reports false
// regardless of Kind, which would make this assertion pass whether or not
// the guard is scoped correctly.
var _ = Describe("coach codesignal (real scan): guided policy authoring guard treats an unclassified ProjectConfigError Kind as non-actionable (AC-POL-8)", func() {
	When("a *ProjectConfigError carries an unclassified Kind, and a controlling terminal is available", func() {
		It("does not select guided authoring", func() {
			master, slave := openPTYPair()
			DeferCleanup(master.Close)
			DeferCleanup(slave.Close)

			originalStdin := os.Stdin
			os.Stdin = slave
			DeferCleanup(func() { os.Stdin = originalStdin })

			var zeroKindErr codesignalcli.ProjectConfigError
			zeroKindErr.Message = `coach codesignal: --project-config "project.json" is invalid at revision "HEAD" (project_config_invalid): unclassified`

			Expect(scanShouldAuthorProjectConfig(&zeroKindErr, "typescript", "project.json", false)).To(BeFalse(), "an error whose cause was never classified must not select guided authoring")
		})
	})
})

// installInvocationCount counts miseDir's logged invocations that began
// with "install", for the single-use-confirmation proof below: a second,
// unread "install" answer left in the pty must never cause a second `mise
// install` to run within the same coach invocation.
func installInvocationCount(miseDir string) int {
	count := 0
	for _, line := range readStubMiseInvocations(miseDir) {
		if strings.HasPrefix(line, "install ") {
			count++
		}
	}
	return count
}

// The interactive compiler-setup offer: a real scan's
// CompilerUnresolvedError gap, on a controlling terminal, presents
// AvailableSetupChoices' menu instead of only printing a remediation line
// (the cancellation half of this offer is pinned above, alongside AC-17's
// guard). This block covers the remaining slices AC-9 requires: setup
// failure, setup success with the AC-SET-6 readiness rerun letting the same
// scan continue (AC-7), and AC-11's single-use-confirmation proof.
var _ = Describe("coach codesignal (real scan): interactive compiler setup offer on a controlling terminal (AC-SET-6/7/8/18, AC-11)", func() {
	When("a mise scope is selected and confirmed, but the mise install itself fails", func() {
		It("exits 2, emits no CodeSignal report, and leaves the repository's tracked files unmodified (AC-18)", func() {
			repo := noSupportedCompilerRepo()
			writeWorktreeFile(repo, "mise.toml", fmt.Sprintf("[tools]\n\"npm:typescript\" = %q\n", codesignalcli.SupportedTypescriptVersions[0]))
			miseDir := writeFailingInstallStubMiseScript()
			path := writeStubNodeScript("v24.9.9") + string(os.PathListSeparator) + miseDir + string(os.PathListSeparator) + pathExcludingToolchain()
			statusBefore := gitStatusPorcelain(repo)

			session := startCoachBinaryWithControllingTerminal(commandPath, repo, stubToolchainEnv(path),
				"codesignal", "--baseline", "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			session.waitForPrompt("an unrecognized or blank answer cancels.")
			session.writeLine("project_mise")
			session.waitForPrompt("Type 'install' to run this setup command now")
			session.writeLine("install")
			stdout, stderr, _, exitCode := session.wait()

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty(), "a failed setup must never let the scan continue to a rendered report")
			Expect(string(stderr)).To(ContainSubstring("compiler setup failed"), "stderr: %s", stderr)
			Expect(miseInvocationsIncludeInstall(miseDir)).To(BeTrue(), "sanity: the confirmed selection must have actually invoked `mise install`")
			Expect(gitStatusPorcelain(repo)).To(Equal(statusBefore), "a failed compiler setup must never modify the repository's own worktree (mise's own install state lives entirely outside it, in miseDir)")
		})
	})

	When("a mise scope is selected and confirmed, and the mise install succeeds", func() {
		It("reruns the complete readiness check (AC-SET-6/AC-7) and, since the rerun reports ready, continues the same scan through to a rendered CodeSignal report", func() {
			repo := noSupportedCompilerRepo()
			writeWorktreeFile(repo, "mise.toml", fmt.Sprintf("[tools]\n\"npm:typescript\" = %q\n", codesignalcli.SupportedTypescriptVersions[0]))
			miseDir := writeStatefulStubMiseScript(codesignalcli.SupportedTypescriptVersions[0])
			path := writeStubNodeScript("v24.9.9") + string(os.PathListSeparator) + miseDir + string(os.PathListSeparator) + pathExcludingToolchain()

			session := startCoachBinaryWithControllingTerminal(commandPath, repo, stubToolchainEnv(path),
				"codesignal", "--baseline", "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			session.waitForPrompt("an unrecognized or blank answer cancels.")
			session.writeLine("project_mise")
			session.waitForPrompt("Type 'install' to run this setup command now")
			session.writeLine("install")
			stdout, stderr, _, exitCode := session.wait()

			Expect(exitCode).To(Equal(0), "the fresh readiness rerun must let this trivial, otherwise-ready fixture's scan continue to completion; stderr: %s", stderr)
			Expect(miseInvocationsIncludeInstall(miseDir)).To(BeTrue())
			// The gap code still appears once, as the diagnosis that opened
			// the offer ("the compiler check reports typescript_compiler_missing").
			// What must never survive a successful setup is the remediation
			// line: "run --check-project" is stale advice the moment the
			// scan resumes, and it is the only form a customer could read as
			// "the compiler is still missing".
			Expect(string(stderr)).NotTo(ContainSubstring("typescript_compiler_missing: run coach"), "a customer whose setup just succeeded must never be told to go and resolve the gap it resolved; stderr: %s", stderr)

			var report map[string]any
			Expect(json.Unmarshal(stdout, &report)).To(Succeed(), "a genuinely continued scan must render a real CodeSignal report as JSON; stdout: %s stderr: %s", stdout, stderr)
			Expect(report).To(HaveKey("coverage"), "the rendered document must be a CodeSignal report, not a project-config candidate or a setup transcript")
		})
	})

	When("a mise scope is selected and confirmed, the install succeeds, and a second unread confirmation answer is left over", func() {
		It("never replays the leftover answer into a second execution within the same run (AC-11's single-use confirmation)", func() {
			repo := noSupportedCompilerRepo()
			writeWorktreeFile(repo, "mise.toml", fmt.Sprintf("[tools]\n\"npm:typescript\" = %q\n", codesignalcli.SupportedTypescriptVersions[0]))
			miseDir := writeStatefulStubMiseScript(codesignalcli.SupportedTypescriptVersions[0])
			path := writeStubNodeScript("v24.9.9") + string(os.PathListSeparator) + miseDir + string(os.PathListSeparator) + pathExcludingToolchain()

			session := startCoachBinaryWithControllingTerminal(commandPath, repo, stubToolchainEnv(path),
				"codesignal", "--baseline", "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			session.waitForPrompt("an unrecognized or blank answer cancels.")
			session.writeLine("project_mise")
			session.waitForPrompt("Type 'install' to run this setup command now")
			session.writeLine("install")
			// A second "install" answer is left unread on the terminal: if the
			// confirmation gate were ever replayable, this would drive a second
			// `mise install` (or reopen the confirmation prompt) within the same
			// coach invocation.
			session.writeLine("install")
			stdout, stderr, _, exitCode := session.wait()

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(stdout).NotTo(BeEmpty())
			Expect(installInvocationCount(miseDir)).To(Equal(1), "the leftover, unread confirmation answer must never cause a second `mise install`")
		})
	})
})

// The combined-menu fixture in project_ts_scan_preflight_acceptance_test.go
// proves project_package and project_mise compose in one real readiness
// result; this spec is what actually confirms and executes the
// project_package half through its own library path
// (BuildSetupPreview/RunConfirmedSetupAndRecheckReadiness), distinct from the
// mise scopes' own install path, which that combined-menu spec explicitly
// leaves unexercised. It lives in this linux-only file, rather than beside
// that fixture, because it drives startCoachBinaryWithControllingTerminal.
var _ = Describe("coach codesignal (real scan): confirming project_package from the combined compiler-setup menu runs npm ci through its own library path (AC-11)", func() {
	When("project_package is selected and confirmed, but the confirmed npm ci itself fails", func() {
		It("previews the exact npm command and working directory, exits 2 with no report, reports the failure, and never invokes mise install", func() {
			nodeDir := writeStubNodeScript("v24.9.9")
			npmDir := writeStubPackageManagerScript("npm", "11.0.0")
			miseDir := writeStatefulStubMiseScript(codesignalcli.SupportedTypescriptVersions[0])
			path := nodeDir + string(os.PathListSeparator) + npmDir + string(os.PathListSeparator) + miseDir + string(os.PathListSeparator) + pathExcludingToolchain()

			repo := newTempGitRepo()
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")
			commitFile(repo, "package.json", fmt.Sprintf(`{"name":"example","version":"1.0.0","devDependencies":{"typescript":%q}}`+"\n", codesignalcli.SupportedTypescriptVersions[0]))
			commitFile(repo, "package-lock.json", `{"name":"example","lockfileVersion":3}`+"\n")
			writeWorktreeFile(repo, "mise.toml", fmt.Sprintf("[tools]\n\"npm:typescript\" = %q\n", codesignalcli.SupportedTypescriptVersions[0]))

			session := startCoachBinaryWithControllingTerminal(commandPath, repo, stubToolchainEnv(path),
				"codesignal", "--baseline", "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			session.waitForPrompt("an unrecognized or blank answer cancels.")
			session.writeLine("project_package")
			session.waitForPrompt("Type 'confirm' to run this setup command now")
			session.writeLine("confirm")
			stdout, stderr, _, exitCode := session.wait()

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty(), "a failed setup must never let the scan continue to a rendered report")

			stderrStr := string(stderr)
			Expect(stderrStr).To(ContainSubstring("Executable: npm"), "the preview must name the exact command project_package resolves to; stderr: %s", stderr)
			Expect(stderrStr).To(ContainSubstring("Arguments: ci --ignore-scripts"), "stderr: %s", stderr)
			Expect(stderrStr).To(ContainSubstring("Working directory: "+repo), "the preview must name the manifest's own working directory; stderr: %s", stderr)
			Expect(stderrStr).To(ContainSubstring("compiler setup failed"), "stderr: %s", stderr)
			Expect(stderrStr).To(ContainSubstring("npm ci --ignore-scripts exited"), "the failure detail must name the command that actually ran and its exit status; stderr: %s", stderr)
			Expect(stderrStr).To(ContainSubstring("the setup command may have changed: mise.toml"), "the residue disclosure must reach the customer repository-relative, never as an absolute path")
			Expect(miseInvocationsIncludeInstall(miseDir)).To(BeFalse(), "confirming project_package must execute only its own library path, never mise install")
		})
	})
})

// runScanCompilerSetupOffer's own retry is the only caller that consumes
// the compiler-setup offer kind, and it does so only after a successful,
// verified install -- a real repository cannot be driven back to the exact
// same compiler gap on that immediate second attempt (every gap code maps to
// needs_prerequisite, so readinessAllowsScan would already have refused to
// retry at all), which is why this bound is exercised directly against
// runCodesignalScan rather than through a full coach invocation: no fixture
// can make a genuine end-to-end retry hit a second gap to prove this against.
var _ = Describe("coach codesignal (real scan): the interactive compiler-setup offer runs at most once per invocation (AC-SET-9)", func() {
	When("runCodesignalScan is called with an empty offer budget, a controlling terminal is available, and the scan hits a real compiler gap that genuinely offers project_package", func() {
		It("never opens the interactive compiler-setup offer, falling through to the plain message-only remediation instead", func() {
			nodeDir := writeStubNodeScript("v24.9.9")
			npmDir := writeStubPackageManagerScript("npm", "11.0.0")
			path := nodeDir + string(os.PathListSeparator) + npmDir + string(os.PathListSeparator) + pathExcludingToolchain()
			GinkgoT().Setenv("PATH", path)
			GinkgoT().Setenv("HOME", os.Getenv("HOME"))
			// Every other controlling-terminal spec in this suite spawns the
			// coach binary as a subprocess with its own explicit env
			// (stubToolchainEnv), which never passes the runner's own CI
			// variable through -- so nonInteractiveRequested inside that
			// child is driven only by --no-interactive/f.noInteractive.
			// This spec instead calls runCodesignalScan in-process, which
			// reads os.Getenv("CI") directly from the test binary's own
			// environment: on a CI runner that ambient CI=true would make
			// nonInteractiveRequested true regardless of the pty this spec
			// sets up, so an empty offer budget would no longer be what
			// withholds the offer, and the appended remediation assertion
			// below would fail for a reason unrelated to what this spec
			// claims to prove.
			GinkgoT().Setenv("CI", "")

			repo := newTempGitRepo()
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")
			commitFile(repo, "package.json", fmt.Sprintf(`{"name":"example","version":"1.0.0","devDependencies":{"typescript":%q}}`+"\n", codesignalcli.SupportedTypescriptVersions[0]))
			commitFile(repo, "package-lock.json", `{"name":"example","lockfileVersion":3}`+"\n")

			master, slave := openPTYPair()
			DeferCleanup(master.Close)
			DeferCleanup(slave.Close)
			originalStdin := os.Stdin
			os.Stdin = slave
			DeferCleanup(func() { os.Stdin = originalStdin })
			// Left unread on the pty: if the empty offer budget were ever ignored, the
			// interactive offer would read this as its selection and cancel
			// (rather than hang the test forever), producing a distinctly
			// different, longer stderr this spec's assertions below catch.
			_, writeErr := master.WriteString("cancel\n")
			Expect(writeErr).NotTo(HaveOccurred())

			outRead, outWrite, err := os.Pipe()
			Expect(err).NotTo(HaveOccurred())
			errRead, errWrite, err := os.Pipe()
			Expect(err).NotTo(HaveOccurred())

			f := codesignalFlags{baseline: true, format: "json", projectConfig: "project.json", projectConfigSet: true, projectLanguage: "typescript"}
			exitCode := runCodesignalScan(repo, f, outWrite, errWrite, scanOfferBudget{})
			Expect(outWrite.Close()).To(Succeed())
			Expect(errWrite.Close()).To(Succeed())
			stdout, readErr := io.ReadAll(outRead)
			Expect(readErr).NotTo(HaveOccurred())
			stderr, readErr := io.ReadAll(errRead)
			Expect(readErr).NotTo(HaveOccurred())

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty())

			lines := stderrLines(stderr)
			Expect(lines).To(HaveLen(1), "an empty offer budget must withhold the interactive offer even though a controlling terminal is available and the gap is genuine; stderr: %s", stderr)
			Expect(lines[0]).To(Equal("typescript_compiler_missing: run coach codesignal --baseline --check-project --project-language typescript --project-config project.json"))
		})
	})
})

// scanShouldOfferCompilerSetup/RunCompilerSetupOffer must never open the
// interactive menu for a runtime-boundary gap (node_missing, node_unsupported):
// gapCodeTable/nextActionExecutable mark every node-related gap as
// permanently instruction-only, exactly like the pre-existing
// --prepare-compiler flow's own PrepareCompilerRemediation gate (D3, above).
// A real repository can independently fail checks.compiler with a genuinely
// installable mise scope while Node itself is absent -- Node resolution and
// compiler resolution are two unrelated CheckProjectReadiness reads
// (project_readiness.go) -- so this is a routine repository shape, not a
// contrived one, and it must still resolve to the plain remediation line
// rather than a prompt (AC-13, AC-SET-10).
var _ = Describe("coach codesignal (real scan): the interactive compiler-setup offer never opens for a runtime-boundary gap (AC-13, AC-SET-10)", func() {
	When("host Node is genuinely missing while the repository's mise scope independently declares an installable TypeScript version, and a controlling terminal is available", func() {
		It("never opens the interactive compiler-setup prompt, printing only the node_missing remediation line and exiting 2", func() {
			repo := noSupportedCompilerRepo()
			writeWorktreeFile(repo, "mise.toml", fmt.Sprintf("[tools]\n\"npm:typescript\" = %q\n", codesignalcli.SupportedTypescriptVersions[0]))
			miseDir := writeStatefulStubMiseScript(codesignalcli.SupportedTypescriptVersions[0])
			path := miseDir + string(os.PathListSeparator) + pathExcludingToolchain()
			requireNodeUnreachable(path)

			session := startCoachBinaryWithControllingTerminal(commandPath, repo, stubToolchainEnv(path),
				"codesignal", "--baseline", "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			stdout, stderr, _, exitCode := session.wait()

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty(), "stdout must stay reserved for the final report; none was produced")
			Expect(string(stderr)).NotTo(ContainSubstring("TypeScript compiler setup:"), "a node_missing gap must never open the interactive compiler-setup menu, regardless of what checks.compiler independently offers")

			lines := stderrLines(stderr)
			Expect(lines).To(HaveLen(1), "stderr: %s", stderr)
			Expect(lines[0]).To(Equal("node_missing: run coach codesignal --baseline --check-project --project-language typescript --project-config project.json"))
			Expect(miseInvocationsIncludeInstall(miseDir)).To(BeFalse(), "withholding the offer must never invoke `mise install`")
		})
	})

	When("host Node major is outside the supported set while the repository's mise scope independently declares an installable TypeScript version, and a controlling terminal is available", func() {
		It("never opens the interactive compiler-setup prompt, printing only the node_unsupported remediation line and exiting 2", func() {
			repo := noSupportedCompilerRepo()
			writeWorktreeFile(repo, "mise.toml", fmt.Sprintf("[tools]\n\"npm:typescript\" = %q\n", codesignalcli.SupportedTypescriptVersions[0]))
			miseDir := writeStatefulStubMiseScript(codesignalcli.SupportedTypescriptVersions[0])
			path := writeStubNodeScript("v25.0.0") + string(os.PathListSeparator) + miseDir + string(os.PathListSeparator) + pathExcludingToolchain()

			session := startCoachBinaryWithControllingTerminal(commandPath, repo, stubToolchainEnv(path),
				"codesignal", "--baseline", "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			stdout, stderr, _, exitCode := session.wait()

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty(), "stdout must stay reserved for the final report; none was produced")
			Expect(string(stderr)).NotTo(ContainSubstring("TypeScript compiler setup:"), "a node_unsupported gap must never open the interactive compiler-setup menu either")

			lines := stderrLines(stderr)
			Expect(lines).To(HaveLen(1), "stderr: %s", stderr)
			Expect(lines[0]).To(Equal("node_unsupported: run coach codesignal --baseline --check-project --project-language typescript --project-config project.json"))
			Expect(miseInvocationsIncludeInstall(miseDir)).To(BeFalse(), "withholding the offer must never invoke `mise install`")
		})
	})
})

// R1: a pty-allocating but genuinely unattended automation context (a real
// controlling terminal with nobody attending it -- docker run -t, ssh -t,
// script -qec, most CI runners with a tty) must never hang reading an
// unanswered interactive prompt. Node present, no TypeScript declared or
// installed, and a mise.toml pinning a genuinely supported version is what
// makes the interactive compiler-setup menu open at all here (unlike the
// node_missing/node_unsupported fixtures above, whose NoChoicesOffered path
// already exits without a prompt regardless of this fix): this fixture is
// the one shape the pre-fix binary actually hangs on.
var _ = Describe("coach codesignal (real scan): --no-interactive and a non-empty CI environment variable prevent an unattended controlling terminal from hanging on the interactive compiler-setup offer (R1)", func() {
	DescribeTable("exits 2 promptly with nobody ever answering the pty, never opens the interactive menu, and produces stderr byte-identical to the no-controlling-terminal (piped) shape for the identical fixture",
		func(extraArgs []string, extraEnv []string) {
			repo := newTempGitRepo()
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			writeWorktreeFile(repo, "mise.toml", fmt.Sprintf("[tools]\n\"npm:typescript\" = %q\n", codesignalcli.SupportedTypescriptVersions[0]))
			miseDir := writeStatefulStubMiseScript(codesignalcli.SupportedTypescriptVersions[0])
			path := writeStubNodeScript("v24.9.9") + string(os.PathListSeparator) + miseDir + string(os.PathListSeparator) + pathExcludingToolchain()

			env := append(append([]string{}, stubToolchainEnv(path)...), extraEnv...)
			args := append([]string{"codesignal", "--baseline", "--project-config", "project.json", "--project-language", "typescript", "--format=json"}, extraArgs...)

			session := startCoachBinaryWithControllingTerminal(commandPath, repo, env, args...)
			stdout, stderr, _, exitCode := session.wait()

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty(), "stdout must stay reserved for the final report; none was produced")
			Expect(string(stderr)).NotTo(ContainSubstring("TypeScript compiler setup:"), "the interactive menu must never open for a non-interactive invocation, even on a real controlling terminal; stderr: %s", stderr)
			Expect(miseInvocationsIncludeInstall(miseDir)).To(BeFalse(), "a non-interactive invocation must never invoke `mise install`")

			pipedStdout, pipedStderr, pipedExitCode := runCoachCodesignalBaselineEnv(repo, path, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(exitCode).To(Equal(pipedExitCode), "a non-interactive controlling-terminal invocation must exit exactly like the no-TTY (piped) shape")
			Expect(stdout).To(Equal(pipedStdout))
			Expect(stderr).To(Equal(pipedStderr), "falling through to the message-only remediation must be byte-identical to the piped shape, not a third, bespoke shape")
		},
		Entry("a non-empty CI environment variable alone, no --no-interactive flag", nil, []string{"CI=1"}),
		Entry("the --no-interactive flag alone, no CI environment variable", []string{"--no-interactive"}, nil),
	)
})

// R1 closed the hang on the scan path, but #330 makes this task the owner of
// the shared no-TTY contract for every surface, and the two standalone
// commands whose entire purpose is prompting were left gated on
// HasControllingTerminal alone. A pty-allocating CI job (docker run -t, ssh
// -t, a tty-allocating runner) that invokes either of them therefore reports
// a genuine controlling terminal with nobody attending it, and blocks on an
// unanswered prompt until the job times out -- with no flag available to
// prevent it, since neither validator accepted --no-interactive at all.
var _ = Describe("coach codesignal: --no-interactive and a non-empty CI environment variable also prevent the standalone interactive commands from hanging on an unattended controlling terminal", func() {
	DescribeTable("--prepare-compiler refuses promptly with nobody ever answering the pty, never opens its menu, and never invokes `mise install`",
		func(extraArgs []string, extraEnv []string) {
			repo := newTempGitRepo()
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			writeWorktreeFile(repo, "mise.toml", fmt.Sprintf("[tools]\n\"npm:typescript\" = %q\n", codesignalcli.SupportedTypescriptVersions[0]))
			miseDir := writeStatefulStubMiseScript(codesignalcli.SupportedTypescriptVersions[0])
			path := writeStubNodeScript("v24.9.9") + string(os.PathListSeparator) + miseDir + string(os.PathListSeparator) + pathExcludingToolchain()

			env := append(append([]string{}, stubToolchainEnv(path)...), extraEnv...)
			args := append([]string{"codesignal", "--baseline", "--prepare-compiler", "--project-language", "typescript", "--project-config", "project.json"}, extraArgs...)

			session := startCoachBinaryWithControllingTerminal(commandPath, repo, env, args...)
			stdout, stderr, _, exitCode := session.wait()

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty(), "this flow never renders a report, so stdout must stay empty")
			Expect(string(stderr)).To(ContainSubstring("refusing to enter interactive compiler setup or mutate mise state"), "a non-interactive invocation must reuse the existing no-controlling-terminal refusal rather than a third, bespoke shape; stderr: %s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("TypeScript compiler setup:"), "the interactive menu must never open for a non-interactive invocation, even on a real controlling terminal; stderr: %s", stderr)
			Expect(miseInvocationsIncludeInstall(miseDir)).To(BeFalse(), "a non-interactive invocation must never invoke `mise install`")
		},
		Entry("a non-empty CI environment variable alone, no --no-interactive flag", nil, []string{"CI=1"}),
		Entry("the --no-interactive flag alone, no CI environment variable", []string{"--no-interactive"}, nil),
	)

	DescribeTable("--suggest-project-config refuses promptly with nobody ever answering the pty and never opens a guided session",
		func(extraArgs []string, extraEnv []string) {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
			path := pathWithStubNode("v24.9.9")

			env := append(append([]string{}, stubToolchainEnv(path)...), extraEnv...)
			args := append([]string{"codesignal", "--baseline", "--suggest-project-config", "--project-language", "typescript"}, extraArgs...)

			session := startCoachBinaryWithControllingTerminal(commandPath, repo, env, args...)
			stdout, stderr, _, exitCode := session.wait()

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty(), "no candidate may be emitted when the guided session never ran")
			Expect(string(stderr)).To(ContainSubstring("refusing to enter guided policy authoring"), "a non-interactive invocation must reuse the existing no-controlling-terminal refusal; stderr: %s", stderr)
		},
		Entry("a non-empty CI environment variable alone, no --no-interactive flag", nil, []string{"CI=1"}),
		Entry("the --no-interactive flag alone, no CI environment variable", []string{"--no-interactive"}, nil),
	)
})

// AC-SET-5: "If package-manager or workspace ownership is ambiguous, then
// Coach shall require an explicit selection with no default." A policy
// selecting two roots that each own their own package.json resolves two
// manifest contexts, and only one of them can be the previewed working
// directory of a single consented install -- so offering project_package at
// all would ask the customer to approve a network install that can satisfy
// at most one selected root. The compiler check requires every selected root
// to resolve the same installed version, so AC-SET-6's mandatory rerun would
// still report the gap: a real mutation, no progress, and no explanation.
var _ = Describe("coach codesignal (real scan): a project-package install whose manifest context is ambiguous is never offered (AC-SET-5)", func() {
	When("the policy selects two roots that each own a separate package.json manifest context, and neither has an installed compiler", func() {
		It("withholds project_package with its reason rather than silently defaulting to the first context, and never opens the setup menu", func() {
			repo := newTempGitRepo()
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["packages/a","packages/b"]}`+"\n")
			manifest := fmt.Sprintf(`{"name":"example","version":"1.0.0","devDependencies":{"typescript":%q}}`+"\n", codesignalcli.SupportedTypescriptVersions[0])
			for _, pkg := range []string{"packages/a", "packages/b"} {
				commitFile(repo, pkg+"/package.json", manifest)
				commitFile(repo, pkg+"/package-lock.json", `{"name":"example","lockfileVersion":3}`+"\n")
				commitFile(repo, pkg+"/tsconfig.json", `{"compilerOptions":{}}`+"\n")
			}

			npmDir := writeRecordingStubPackageManagerScript("npm", "11.0.0")
			path := writeStubNodeScript("v24.9.9") + string(os.PathListSeparator) + npmDir + string(os.PathListSeparator) + pathExcludingToolchain()

			stdout, stderr, _, exitCode := runCoachBinaryWithControllingTerminal(commandPath, repo, stubToolchainEnv(path), "",
				"codesignal", "--baseline", "--project-config", "project.json", "--project-language", "typescript", "--format=json")

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty(), "stdout must stay reserved for the final report; none was produced")
			Expect(string(stderr)).NotTo(ContainSubstring("TypeScript compiler setup:"), "an install that can satisfy at most one of two selected roots must never be offered at all; stderr: %s", stderr)
			Expect(string(stderr)).To(ContainSubstring("manifest_context_ambiguous"), "the customer must be told why the project-package choice was ruled out, not silently handed a default; stderr: %s", stderr)
		})
	})
})

// Withholding project_package from a menu that still has entries is different
// from never building it: the customer sees a shorter menu, and without a
// reason that reads as "Coach cannot install from my package manager". The
// fixture below is the one shape where both halves matter -- two manifest
// contexts make the project-package install unserviceable, while a verified
// mise scope keeps the menu open.
var _ = Describe("coach codesignal (real scan): a withheld project_package choice says why, when the menu still offers something else (AC-SET-1)", func() {
	When("the policy selects two roots with separate manifests and a verified mise scope can still install the compiler", func() {
		It("opens the menu without project_package and names the reason it was ruled out", func() {
			repo := newTempGitRepo()
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["packages/a","packages/b"]}`+"\n")
			manifest := fmt.Sprintf(`{"name":"example","version":"1.0.0","devDependencies":{"typescript":%q}}`+"\n", codesignalcli.SupportedTypescriptVersions[0])
			for _, pkg := range []string{"packages/a", "packages/b"} {
				commitFile(repo, pkg+"/package.json", manifest)
				commitFile(repo, pkg+"/package-lock.json", `{"name":"example","lockfileVersion":3}`+"\n")
				commitFile(repo, pkg+"/tsconfig.json", `{"compilerOptions":{}}`+"\n")
			}
			writeWorktreeFile(repo, "mise.toml", fmt.Sprintf("[tools]\n\"npm:typescript\" = %q\n", codesignalcli.SupportedTypescriptVersions[0]))

			npmDir := writeRecordingStubPackageManagerScript("npm", "11.0.0")
			miseDir := writeStatefulStubMiseScript(codesignalcli.SupportedTypescriptVersions[0])
			path := writeStubNodeScript("v24.9.9") + string(os.PathListSeparator) + npmDir + string(os.PathListSeparator) + miseDir + string(os.PathListSeparator) + pathExcludingToolchain()

			session := startCoachBinaryWithControllingTerminal(commandPath, repo, stubToolchainEnv(path),
				"codesignal", "--baseline", "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			session.waitForPrompt("an unrecognized or blank answer cancels.")
			session.writeLine("cancel")
			stdout, stderr, _, exitCode := session.wait()

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty(), "stdout must stay reserved for the final report; none was produced")
			Expect(string(stderr)).To(ContainSubstring("project_package is not offered here (manifest_context_ambiguous)"), "a choice removed from a menu the customer can still see must say why; stderr: %s", stderr)
			Expect(string(stderr)).To(ContainSubstring("- project_mise"), "the surviving mise choice must still be offered; stderr: %s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("  - project_package"), "the unserviceable choice must not appear in the menu itself; stderr: %s", stderr)
			Expect(miseInvocationsIncludeInstall(miseDir)).To(BeFalse(), "cancelling must never invoke `mise install`")
		})
	})
})
