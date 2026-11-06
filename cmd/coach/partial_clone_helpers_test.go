package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func gitOutputIn(dir string, args ...string) string {
	command := exec.Command("git", args...)
	command.Dir = dir
	command.Env = commitEnv
	output, err := command.CombinedOutput()
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "git %s: %s", strings.Join(args, " "), output)
	return string(output)
}

// commitPartialClone is a clone made with --filter=blob:none and --no-checkout, so the blob of the
// committed policy at cfg/p.json is not local and git would fetch it from the promisor remote. The
// remote is then set to remoteURL(bare), a program to run or a repository that would answer.
func commitPartialClone(remoteURL func(bare string) string) string {
	source := commitScannableGoRepo()
	commitFile(source, "cfg/p.json", singleRootPolicyJSON)
	bare := filepath.Join(GinkgoT().TempDir(), "bare.git")
	gitOutputIn(source, "clone", "--bare", "--quiet", source, bare)
	gitOutputIn(bare, "config", "uploadpack.allowFilter", "true")
	clone := filepath.Join(GinkgoT().TempDir(), "part")
	gitOutputIn(GinkgoT().TempDir(), "clone", "--quiet", "--no-checkout", "--filter=blob:none", "file://"+bare, clone)
	gitOutputIn(clone, "remote", "set-url", "origin", remoteURL(bare))
	return clone
}

// sshProgramThatLeavesAMarker is the value for GIT_SSH_COMMAND, and the marker it leaves when git runs it.
func sshProgramThatLeavesAMarker() (command, marker string) {
	scripts := GinkgoT().TempDir()
	marker = filepath.Join(scripts, "ssh-ran")
	command = filepath.Join(scripts, "fakessh")
	Expect(os.WriteFile(command, []byte("#!/bin/sh\ntouch '"+marker+"'\nexit 1\n"), 0o755)).To(Succeed())
	return command, marker
}

func packFilesIn(clone string) int {
	entries, err := os.ReadDir(filepath.Join(clone, ".git", "objects", "pack"))
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	return len(entries)
}
