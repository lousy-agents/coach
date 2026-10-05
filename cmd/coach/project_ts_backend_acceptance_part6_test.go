package main

import (
	"os"
	"os/exec"

	"strconv"
	"strings"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

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

func startAnalyzerEnvironSampler() *analyzerEnvironSampler {
	s := newIdleAnalyzerEnvironSampler()
	go func() {
		body_projectTsBackendAcceptancePart6Test_37(s)
	}()
	return s
}

func hasStartTimeKeyedObservation(seen map[string]struct{}, pidKey string) bool {
	prefix := pidKey + ":"
	for key := range seen {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

func containsProjectModelDiagnosticCode(diagnostics []projectmodel.Diagnostic, code string) bool {
	for _, d := range diagnostics {
		if d.Code == code {
			return true
		}
	}
	return false
}

// countProjectModelDiagnosticCode asserts a model diagnostic is folded into
// the reported ProjectCoverage exactly once (see tsBypassCoverageForFold in
// internal/codesignalcli/project_ts_backend.go), not once per fold.
func countProjectModelDiagnosticCode(diagnostics []projectmodel.Diagnostic, code string) int {
	count := 0
	for _, d := range diagnostics {
		if d.Code == code {
			count++
		}
	}
	return count
}
