package main

import (
	"bytes"
	"encoding/json"
	"os/exec"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// runCoachCodesignalRaw runs `coach codesignal --base <base> [extraArgs...]`
// in repo, returning raw stdout/stderr without assuming success or a
// particular --format.
func runCoachCodesignalRaw(repo, base string, extraArgs ...string) (stdout, stderr []byte, exitCode int) {
	args := append([]string{"codesignal", "--base", base}, extraArgs...)
	return runCoachBinary(commandPath, repo, nil, args...)
}

func runCoachRaw(args ...string) (stdout, stderr []byte, exitCode int) {
	return runCoachBinary(commandPath, "", nil, args...)
}

func removeFile(repo, name string) string {
	rmCmd := exec.Command("git", "rm", name)
	rmCmd.Dir = repo
	output, err := rmCmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "git rm: %s", output)

	commitCmd := exec.Command("git", "commit", "-m", "remove "+name)
	commitCmd.Dir = repo
	commitCmd.Env = commitEnv
	output, err = commitCmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "git commit: %s", output)

	revCmd := exec.Command("git", "rev-parse", "HEAD")
	revCmd.Dir = repo
	output, err = revCmd.Output()
	Expect(err).NotTo(HaveOccurred())

	return string(bytes.TrimSpace(output))
}

func runCoachCodesignal(repo, base string) (*codesignal.Report, string) {
	command := exec.Command(commandPath, "codesignal", "--base", base, "--format=json")
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
