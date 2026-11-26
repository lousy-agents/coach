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
	scan     string
	findings []observedFinding
}

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
	args := []string{"codesignal", "--baseline", "--no-interactive", "--format", "json"}
	if _, err := os.Stat(filepath.Join(work, policyPath)); err == nil {
		args = append(args, "--project-config", policyPath)
	}
	output, err := runIn(work, nil, "", coach, args...)
	if err != nil {
		return scanResult{}, fmt.Errorf("fixture %s: coach failed: %v: %s", label, err, strings.TrimSpace(output))
	}
	findings, err := readFindings(output)
	if err != nil {
		return scanResult{}, fmt.Errorf("fixture %s: %v", label, err)
	}
	return scanResult{scan: strings.Join(args, " "), findings: findings}, nil
}

func readFindings(report string) ([]observedFinding, error) {
	var parsed struct {
		Signals *[]observedFinding `json:"signals"`
	}
	if err := json.Unmarshal([]byte(report), &parsed); err != nil {
		return nil, fmt.Errorf("the coach report is not JSON: %v", err)
	}
	if parsed.Signals == nil {
		return nil, fmt.Errorf("the coach report has no signals key")
	}
	return *parsed.Signals, nil
}
