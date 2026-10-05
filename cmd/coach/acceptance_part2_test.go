package main

import (
	"bytes"
	"encoding/json"
	"errors"

	"os/exec"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func diagnosticFor(report *codesignal.Report, kind, path string) (codesignal.Diagnostic, bool) {
	for _, d := range report.Diagnostics {
		if d.Kind == kind && d.Path == path {
			return d, true
		}
	}
	return codesignal.Diagnostic{}, false
}

func hasDiagnostic(report *codesignal.Report, kind, path string) bool {
	for _, d := range report.Diagnostics {
		if d.Kind == kind && d.Path == path {
			return true
		}
	}
	return false
}

// sourceScopeForPath reads the customer-facing source_scope emitted with a
// signal. It intentionally decodes the public JSON document rather than a Go
// report type so this acceptance suite requires the label to be serialized.
func sourceScopeForPath(stdout []byte, path string) string {
	var document struct {
		Signals []struct {
			Path        string `json:"path"`
			SourceScope string `json:"source_scope"`
		} `json:"signals"`
	}
	Expect(json.Unmarshal(stdout, &document)).To(Succeed(), "stdout should be a JSON CodeSignal report: %s", stdout)

	for _, signal := range document.Signals {
		if signal.Path == path {
			return signal.SourceScope
		}
	}
	return ""
}

// runCoachCodesignalRaw runs `coach codesignal --base <base> [extraArgs...]`
// in repo, returning raw stdout/stderr without assuming success or a
// particular --format.
func runCoachCodesignalRaw(repo, base string, extraArgs ...string) (stdout, stderr []byte, exitCode int) {
	args := append([]string{"codesignal", "--base", base}, extraArgs...)
	command := exec.Command(commandPath, args...)
	command.Dir = repo
	var outBuf, errBuf bytes.Buffer
	command.Stdout = &outBuf
	command.Stderr = &errBuf

	err := command.Run()
	if err == nil {
		return outBuf.Bytes(), errBuf.Bytes(), 0
	}

	var exitErr *exec.ExitError
	Expect(errors.As(err, &exitErr)).To(BeTrue(), "expected an ExitError, got: %s (stderr: %s)", err, errBuf.String())
	return outBuf.Bytes(), errBuf.Bytes(), exitErr.ExitCode()
}

func runCoachRaw(args ...string) (stdout, stderr []byte, exitCode int) {
	command := exec.Command(commandPath, args...)
	var outBuf, errBuf bytes.Buffer
	command.Stdout = &outBuf
	command.Stderr = &errBuf

	err := command.Run()
	if err == nil {
		return outBuf.Bytes(), errBuf.Bytes(), 0
	}

	var exitErr *exec.ExitError
	Expect(errors.As(err, &exitErr)).To(BeTrue(), "expected an ExitError, got: %s (stderr: %s)", err, errBuf.String())
	return outBuf.Bytes(), errBuf.Bytes(), exitErr.ExitCode()
}
