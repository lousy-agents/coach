package main

import (
	"bytes"
	"context"
	"encoding/json"

	"os"

	"strconv"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
)

// analyzerChildPIDsFromProc restricts matches to descendants of this test
// binary's own process. go test ./... runs internal/codesignalcli and
// pkg/projectmodel acceptance suites concurrently, and they spawn their own
// analyzer children with the same --compiler-module= marker; without the
// ancestry check those foreign pids get counted alongside this package's,
// inflating the per-invocation counts these specs assert against.
func analyzerChildPIDsFromProc() ([]int, bool) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, false
	}
	self := os.Getpid()
	var pids []int
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		data, err := os.ReadFile("/proc/" + entry.Name() + "/cmdline")
		if err != nil {
			continue
		}
		if !bytes.Contains(data, []byte(analyzerChildArgMarker)) {
			continue
		}
		if !isDescendantOfProcess(pid, self) {
			continue
		}
		pids = append(pids, pid)
	}
	return pids, true
}

// processParentPID reads a process's parent PID from /proc/<pid>/stat. The
// comm field can itself contain spaces and parentheses, so the parse anchors
// on the stat format's guaranteed last ')' rather than splitting on spaces.
func processParentPID(pid int) (int, bool) {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return 0, false
	}
	idx := bytes.LastIndexByte(data, ')')
	if idx < 0 || idx+2 >= len(data) {
		return 0, false
	}
	fields := strings.Fields(string(data[idx+2:]))
	if len(fields) < 2 {
		return 0, false
	}
	ppid, err := strconv.Atoi(fields[1])
	if err != nil {
		return 0, false
	}
	return ppid, true
}

func (s *analyzerEnvironSampler) capture() {
	for _, pid := range analyzerChildPIDs() {
		env, ok := readProcessEnviron(pid)
		s.record(pid, processStartTime(pid), env, ok)
	}
}

// analyzeTSProjectBackend calls the exported tsProjectBackend contract
// (NewTSProjectBackend/ProjectBackend.Analyze) directly, in-process, rather
// than through the compiled coach binary: project_scope is not yet rendered
// through codesignal.Input/Report (issue #332 Task 10's job, not Task 9
// T1's), so ProjectBackendResult -- the public contract at this boundary --
// is the most meaningful place to observe HeadProjectScope/BaseProjectScope.
// Calling Analyze in-process still spawns the real analyzer subprocess
// (BuildTypeScriptModelViaSidecar), and the analyzer child is still a
// descendant of this test binary, so startAnalyzerEnvironSampler's
// descendant-restricted PID scan observes it exactly as it would through the
// compiled binary.
func analyzeTSProjectBackend(dir, headRevision, baseRevision string, baseline bool, configJSON string) (*codesignalcli.ProjectBackendResult, error) {
	config := json.RawMessage(configJSON)
	backend := codesignalcli.NewTSProjectBackend()
	return backend.Analyze(context.Background(), codesignalcli.ProjectBackendRequest{
		Dir:          dir,
		HeadRevision: headRevision,
		BaseRevision: baseRevision,
		Baseline:     baseline,
		ConfigPath:   "project.json",
		Config:       config,
		ConfigDigest: projectconfig.Digest(config),
		Language:     "typescript",
	})
}
