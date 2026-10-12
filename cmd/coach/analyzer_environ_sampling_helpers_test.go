package main

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func startAnalyzerEnvironSampler() *analyzerEnvironSampler {
	s := newIdleAnalyzerEnvironSampler()
	go s.sampleUntilStopped()
	return s
}

func (s *analyzerEnvironSampler) sampleUntilStopped() {
	defer close(s.done)
	s.capture()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-s.stop:
			s.capture()
			return
		case <-ticker.C:
			s.capture()
		}
	}
}

func (s *analyzerEnvironSampler) capture() {
	for _, pid := range analyzerChildPIDs() {
		env, ok := readProcessEnviron(pid)
		s.record(pid, processStartTime(pid), env, ok)
	}
}

// readProcessEnviron reports a process's environment, or false when none
// could be observed. A successful read of zero bytes is not an observation:
// the kernel returns an empty environ for a task that has already torn down
// its address space, so a child that exits between being listed and being
// read would otherwise be recorded as having no PATH at all.
func readProcessEnviron(pid int) (string, bool) {
	if data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/environ"); err == nil {
		if len(data) == 0 {
			return "", false
		}
		return strings.ReplaceAll(string(data), "\x00", "\n"), true
	}

	out, err := exec.Command("ps", "eww", "-p", strconv.Itoa(pid)).Output()
	if err != nil || !strings.Contains(string(out), "PATH=") {
		return "", false
	}
	return string(out), true
}
