package evaldetect

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
)

const (
	cognitiveComplexity = "complexity.cognitive_complexity"
	gocognitModule      = "github.com/uudashr/gocognit"
	// gocognit reports complexity strictly above -over; Coach flags 15 and up.
	gocognitOver = "14"
)

// Gocognit runs a pinned gocognit binary.
type Gocognit struct {
	Binary  string
	Version string
}

type gocognitStat struct {
	FuncName string
	Pos      struct {
		Filename string
		Line     int
	}
}

// Findings reports every function under root whose cognitive complexity is at
// or above Coach's threshold.
func (g Gocognit) Findings(root string) ([]Finding, error) {
	if err := g.CheckVersion(); err != nil {
		return nil, err
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	var stdout, stderr bytes.Buffer
	command := exec.Command(g.Binary, "-json", "-over", gocognitOver, absRoot)
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil && !reportsFindings(err, stdout.Len()) {
		return nil, fmt.Errorf("run gocognit: %w: %s", err, stderr.String())
	}
	return parseGocognit(absRoot, stdout.Bytes())
}

// reportsFindings accepts exit status 1 with output, which gocognit uses to
// signal that -over matched something.
func reportsFindings(err error, outputLen int) bool {
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr) && exitErr.ExitCode() == 1 && outputLen > 0
}

func parseGocognit(absRoot string, output []byte) ([]Finding, error) {
	var stats []gocognitStat
	if len(bytes.TrimSpace(output)) > 0 {
		if err := json.Unmarshal(output, &stats); err != nil {
			return nil, fmt.Errorf("parse gocognit output: %w", err)
		}
	}
	findings := make([]Finding, 0, len(stats))
	for _, stat := range stats {
		rel, err := filepath.Rel(absRoot, stat.Pos.Filename)
		if err != nil {
			return nil, err
		}
		findings = append(findings, Finding{RuleID: cognitiveComplexity, Path: filepath.ToSlash(rel), Subject: stat.FuncName, Line: stat.Pos.Line})
	}
	return findings, nil
}
