//go:build linux

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"golang.org/x/sys/unix"
)

// syncBuffer is a goroutine-safe bytes.Buffer: os/exec reads a command's
// Stdout/Stderr pipes on their own internal goroutines, so a spec polling
// the same buffer from the test goroutine (waitForPrompt) needs its
// own synchronization -- a plain bytes.Buffer assigned directly as
// cmd.Stdout/Stderr is not safe for that concurrent read.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

// runCoachBinaryWithControllingTerminal runs binary with its stdin attached
// to a real controlling terminal (a pty slave) instead of a pipe, so
// terminal.HasControllingTerminal(os.Stdin) is genuinely true inside
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

// openPTYPair opens a real Linux pseudo-terminal pair via /dev/ptmx,
// duplicating internal/codesignalcli/terminal/controlling_terminal_pty_linux_test.go's
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
