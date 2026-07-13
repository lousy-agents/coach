package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var commandPath string

func TestCoachAcceptance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "coach command acceptance suite")
}

var _ = BeforeSuite(func() {
	directory, err := os.MkdirTemp("", "coach-acceptance-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, directory)
	commandPath = filepath.Join(directory, "coach")
	build := exec.Command("go", "build", "-o", commandPath, ".")
	output, err := build.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "building the command: %s", output)
})

func newTempGitRepo() string {
	directory, err := os.MkdirTemp("", "coach-acceptance-repo-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, directory)

	initCmd := exec.Command("git", "init")
	initCmd.Dir = directory
	output, err := initCmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "git init: %s", output)

	return directory
}
