package evalharness

import (
	"encoding/json"
	"fmt"
	"io/fs"
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
	return scanResult{scan: strings.Join(args, " "), findings: findings}, err
}

func readFindings(report string) ([]observedFinding, error) {
	var parsed struct {
		Signals []observedFinding `json:"signals"`
	}
	if err := json.Unmarshal([]byte(report), &parsed); err != nil {
		return nil, fmt.Errorf("the coach report is not JSON: %v", err)
	}
	return parsed.Signals, nil
}

// materialise copies a variant into dir, reading a source stored with a .txt suffix (main.go.txt, go.mod.txt) as the
// source itself, so that a deliberately bad variant is not Go code in the repository that holds it.
func materialise(from, to string) error {
	return filepath.WalkDir(from, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(from, path)
		target := filepath.Join(to, strippedSuffix(rel))
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, content, 0o644)
	})
}

func strippedSuffix(rel string) string {
	base, found := strings.CutSuffix(rel, ".txt")
	if found && (strings.HasSuffix(base, ".go") || strings.HasSuffix(base, "go.mod") || strings.HasSuffix(base, "go.sum")) {
		return base
	}
	return rel
}

func commitAll(dir string) error {
	for _, step := range [][]string{
		{"init", "-q"}, {"add", "-A"},
		{"-c", "user.name=coach-eval", "-c", "user.email=coach-eval@example.invalid", "commit", "-q", "-m", "fixture"},
	} {
		if output, err := runIn(dir, nil, "", "git", step...); err != nil {
			return fmt.Errorf("git %s: %v: %s", step[0], err, output)
		}
	}
	return nil
}
