//go:build linux

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// controllingTerminalCommandTimeout bounds runCoachBinaryWithControllingTerminal's
// child. Without a deadline, a prompt sequence the caller's stdinScript does
// not fully answer leaves the child blocked reading the still-open pty
// forever, and the failure only ever surfaces as the whole package's 10-minute
// test timeout rather than a named spec failure.
const controllingTerminalCommandTimeout = 15 * time.Second

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
	go session.drainTranscript()
	return session
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

// drainTranscript copies the pty master's output into the session
// transcript until the master reports an error (the child closed its end).
func (s *controllingTerminalSession) drainTranscript() {
	defer close(s.drained)
	buf := make([]byte, 4096)
	for {
		n, readErr := s.master.Read(buf)
		if n > 0 {
			s.mu.Lock()
			s.transcript.Write(buf[:n])
			s.mu.Unlock()
		}
		if readErr != nil {
			return
		}
	}
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

func (s *controllingTerminalSession) transcriptSoFar() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.transcript.String()
}

// writeLine sends line plus a trailing newline to the child's controlling
// terminal, as if a person had typed it and pressed enter.
func (s *controllingTerminalSession) writeLine(line string) {
	_, err := s.master.WriteString(line + "\n")
	Expect(err).NotTo(HaveOccurred(), "writing %q to the pty master", line)
}
