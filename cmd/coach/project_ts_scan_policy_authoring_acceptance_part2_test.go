//go:build linux

package main

import (
	"bytes"
	"context"

	"errors"

	"io"
	"os/exec"

	"syscall"

	. "github.com/onsi/gomega"
)

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
		body_projectTsScanPolicyAuthoringAcceptancePart2Test_51(master, session)
	}()
	return session
}

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
