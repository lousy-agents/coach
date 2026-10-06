package main

import (
	"bytes"
	"encoding/json"
	"os/exec"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// runCoachCodesignalBaselineRaw runs `coach codesignal --baseline
// [extraArgs...]` in repo, returning raw stdout/stderr without assuming
// success or a particular --format.
func runCoachCodesignalBaselineRaw(repo string, extraArgs ...string) (stdout, stderr []byte, exitCode int) {
	args := append([]string{"codesignal", "--baseline"}, extraArgs...)
	return runCoachBinary(commandPath, repo, nil, args...)
}

// runCoachCodesignalBaseline builds and runs `coach codesignal --baseline
// --format=json` in repo, decoding stdout as one codesignal.Report.
func runCoachCodesignalBaseline(repo string) (*codesignal.Report, string) {
	command := exec.Command(commandPath, "codesignal", "--baseline", "--format=json")
	command.Dir = repo
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	err := command.Run()
	Expect(err).NotTo(HaveOccurred(), "stderr: %s", stderr.String())

	var report codesignal.Report
	Expect(json.Unmarshal(stdout.Bytes(), &report)).To(Succeed(), "stdout should be one JSON report: %s", stdout.String())

	return &report, stderr.String()
}
