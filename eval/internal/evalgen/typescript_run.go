package evalgen

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

// outputGrace is how long a process that outlives its parent may keep the run's output open.
const outputGrace = 2 * time.Second

func (t TypeScriptTools) limit() time.Duration {
	if t.Limit > 0 {
		return t.Limit
	}
	return DefaultValidationLimit
}

// run executes one tool in its own process group and kills the group when the time limit passes, so a hung or
// background-spawning tool cannot stall generation.
func (t TypeScriptTools) run(dir, program string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), t.limit())
	defer cancel()
	var output bytes.Buffer
	cmd := exec.CommandContext(ctx, program, args...)
	cmd.Dir = dir
	cmd.Stdout, cmd.Stderr = &output, &output
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = outputGrace
	err := cmd.Run()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return output.String(), fmt.Errorf("%s did not finish within the time limit of %s", filepath.Base(program), t.limit())
	}
	return output.String(), err
}
