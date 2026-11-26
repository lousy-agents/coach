package evalgen_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

func validateWith(t *testing.T, node string, limit time.Duration) (error, time.Duration) {
	t.Helper()
	resolved := tools(t)
	resolved.Node, resolved.Limit = node, limit
	started := time.Now()
	err := evalgen.ValidateTypeScript(program(t), resolved)
	return err, time.Since(started)
}

func validateTSCWith(t *testing.T, tsc string, limit time.Duration) (error, time.Duration) {
	t.Helper()
	resolved := tools(t)
	resolved.TSC, resolved.Limit = tsc, limit
	started := time.Now()
	err := evalgen.ValidateTypeScript(program(t), resolved)
	return err, time.Since(started)
}

func TestAValidationRunThatOutlastsItsLimitIsKilledAndSaysSo(t *testing.T) {
	node := writeScript(t, t.TempDir(), "node", "sleep 60")

	err, took := validateWith(t, node, 2*time.Second)

	if err == nil || !strings.Contains(err.Error(), "time limit") {
		t.Fatalf("got %v, want an error that names the time limit", err)
	}
	if took > 30*time.Second {
		t.Fatalf("validation took %s: the limit did not stop the run", took)
	}
}

func TestAValidationRunTscThatOutlastsItsLimitIsKilledAndSaysSo(t *testing.T) {
	tsc := writeScript(t, t.TempDir(), "tsc", "sleep 60")

	err, took := validateTSCWith(t, tsc, 2*time.Second)

	if err == nil || !strings.Contains(err.Error(), "time limit") {
		t.Fatalf("got %v, want an error that names the time limit", err)
	}
	if took > 30*time.Second {
		t.Fatalf("validation took %s: the limit did not stop tsc", took)
	}
}

func TestAValidationRunWhoseLeftoverProcessHoldsItsOutputIsNotHeldOpen(t *testing.T) {
	node := writeScript(t, t.TempDir(), "node", "sleep 60 &\nexit 0")

	_, took := validateWith(t, node, 2*time.Second)

	if took > 30*time.Second {
		t.Fatalf("validation took %s: a background process held the run open", took)
	}
}

func TestAValidationRunKillsTheWholeProcessGroupWhenItsLimitPasses(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "child.pid")
	tsc := writeScript(t, dir, "tsc", "sleep 300 &\necho $! > "+pidFile+"\nsleep 60")

	err, _ := validateTSCWith(t, tsc, 2*time.Second)
	t.Cleanup(func() {
		if pid, ok := pidFrom(pidFile); ok {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	})

	if err == nil || !strings.Contains(err.Error(), "time limit") {
		t.Fatalf("got %v, want an error that names the time limit", err)
	}
	pid := readPID(t, pidFile)
	if !eventuallyGone(pid, 5*time.Second) {
		t.Fatalf("process %d, started by tsc, outlived the run", pid)
	}
}

func TestAZeroLimitMeansTheDefaultNotNoTime(t *testing.T) {
	node := writeScript(t, t.TempDir(), "node", "sleep 1\nexit 0")

	err, _ := validateWith(t, node, 0)

	if err != nil && strings.Contains(err.Error(), "time limit") {
		t.Fatalf("a node that takes one second hit the limit: %v", err)
	}
}

func TestTheDefaultLimitIsOneMinute(t *testing.T) {
	if evalgen.DefaultValidationLimit != time.Minute {
		t.Fatalf("default limit is %s", evalgen.DefaultValidationLimit)
	}
}

func readPID(t *testing.T, path string) int {
	t.Helper()
	pid, ok := pidFrom(path)
	if !ok {
		t.Fatalf("the script did not record its child's pid in %s", path)
	}
	return pid
}

// pidFrom reads the pid a script wrote, if it wrote one.
func pidFrom(path string) (int, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	return pid, err == nil
}

func eventuallyGone(pid int, within time.Duration) bool {
	deadline := time.Now().Add(within)
	for {
		if processGone(pid) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// processGone reports whether pid has exited. A process that is only a zombie has exited too, and a host
// without /proc cannot show that, so the process counts as alive there.
func processGone(pid int) bool {
	if err := syscall.Kill(pid, 0); err != nil {
		return errors.Is(err, syscall.ESRCH)
	}
	stat, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return false
	}
	return strings.Contains(string(stat), ") Z ")
}
