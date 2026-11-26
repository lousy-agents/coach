package evalgrade

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// settleGrace is how long anything holding the run's pipes may outlive node or the time limit. A test can leave a
// background process that keeps node's streams or a copy of the record channel open, and the time limit, not that
// process, decides when a run ends.
const settleGrace = 2 * time.Second

const killedAtTimeLimit = "the run reached its time limit"

// supervised is one node run: the command, the context whose expiry kills it, and the record channel's two ends.
type supervised struct {
	cmd    *exec.Cmd
	ctx    context.Context
	reads  *os.File
	writes *os.File
}

// nodeCommand builds node in its own process group, with the context's expiry killing that whole group, so a process
// the test started without detaching dies with the run.
func nodeCommand(ctx context.Context, node, dir string, args []string, inherited []*os.File, output io.Writer) *exec.Cmd {
	cmd := exec.CommandContext(ctx, node, args...)
	cmd.Dir = dir
	cmd.ExtraFiles = inherited
	cmd.Stdout, cmd.Stderr = output, output
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return killGroup(cmd) }
	cmd.WaitDelay = settleGrace
	return cmd
}

func killGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}

// run starts node, reads its record channel, and returns once node has finished or been killed at the time limit.
func (s supervised) run() (TypeScriptRun, error) {
	reading := make(chan TypeScriptRun, 1)
	err := s.cmd.Start()
	_ = s.writes.Close()
	if err != nil {
		return TypeScriptRun{}, err
	}
	go func() { reading <- readRecords(s.reads) }()
	_ = s.cmd.Wait()
	run := collectRecords(reading, s.reads)
	run.KillReason = s.killReason()
	return run, nil
}

func (s supervised) killReason() string {
	if errors.Is(s.ctx.Err(), context.DeadlineExceeded) {
		return killedAtTimeLimit
	}
	return ""
}

func collectRecords(reading <-chan TypeScriptRun, channel *os.File) TypeScriptRun {
	select {
	case run := <-reading:
		return run
	case <-time.After(settleGrace):
		_ = channel.Close()
		return <-reading
	}
}
