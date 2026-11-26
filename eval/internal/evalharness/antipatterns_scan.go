package evalharness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const policyPath = ".lousy-agents/coach.json"

// observedFinding is a signal Coach reported on a variant.
type observedFinding = expectedFinding

// scanResult is what scanning one variant found, and the arguments that found it.
type scanResult struct {
	scan        string
	findings    []observedFinding
	complete    bool
	diagnostics []string
}

// coachIncomplete is the exit status of coach codesignal --fail-on-incomplete-coverage when coverage was incomplete.
const coachIncomplete = 3

// scanVariant copies a variant, checks that it builds and passes go vet, commits it, and scans its baseline.
func scanVariant(coach, fixture, variant string) (scanResult, error) {
	work, err := os.MkdirTemp("", "coach-eval-antipattern")
	if err != nil {
		return scanResult{}, err
	}
	defer os.RemoveAll(work)
	label := filepath.Base(fixture) + "/" + filepath.Base(variant)
	if err := materialise(variant, work); err != nil {
		return scanResult{}, fmt.Errorf("fixture %s: %v", label, err)
	}
	if output, err := runIn(work, []string{"GOFLAGS=-mod=mod", "GOTOOLCHAIN=local", "GOPROXY=off", "GOWORK=off"}, "", "go", "vet", "./..."); err != nil {
		return scanResult{}, fmt.Errorf("fixture %s: go vet failed: %v: %s", label, err, strings.TrimSpace(output))
	}
	if err := commitAll(work); err != nil {
		return scanResult{}, fmt.Errorf("fixture %s: %v", label, err)
	}
	args := []string{"codesignal", "--baseline", "--no-interactive", "--fail-on-incomplete-coverage", "--format", "json"}
	if _, err := os.Stat(filepath.Join(work, policyPath)); err == nil {
		args = append(args, "--project-config", policyPath)
	}
	output, err := runIn(work, nil, "", coach, args...)
	status := antipatternExit(err)
	if status != 0 && status != coachIncomplete {
		return scanResult{}, fmt.Errorf("fixture %s: coach failed: %v: %s", label, err, strings.TrimSpace(output))
	}
	report, err := readReport(output)
	if err != nil {
		return scanResult{}, fmt.Errorf("fixture %s: %v", label, err)
	}
	return scanResult{scan: strings.Join(args, " "), findings: *report.Signals, complete: status == 0, diagnostics: report.kinds()}, nil
}

// coachReport is the part of a coach report the table reads.
type coachReport struct {
	Signals     *[]observedFinding `json:"signals"`
	Diagnostics []struct {
		Kind string `json:"kind"`
	} `json:"diagnostics"`
}

func (r coachReport) kinds() []string {
	kinds := []string{}
	for _, diagnostic := range r.Diagnostics {
		kinds = append(kinds, diagnostic.Kind)
	}
	return kinds
}

func readReport(report string) (coachReport, error) {
	var parsed coachReport
	if err := json.Unmarshal([]byte(report), &parsed); err != nil {
		return coachReport{}, fmt.Errorf("the coach report is not JSON: %v", err)
	}
	if parsed.Signals == nil {
		return coachReport{}, fmt.Errorf("the coach report has no signals key")
	}
	return parsed, nil
}
