package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
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
	return partialCloneOf(source, remoteURL)
}

// partialCloneOf is a filtered, no-checkout clone of source, with its remote set to remoteURL(bare).
func partialCloneOf(source string, remoteURL func(bare string) string) string {
	return filteredCloneOf(source, remoteURL, "--no-checkout")
}

// completePartialCloneOf is a filtered clone of source whose checkout fetched every blob of HEAD, so HEAD
// lacks nothing and the blobs of earlier commits that differ from it are not local.
func completePartialCloneOf(source string, remoteURL func(bare string) string) string {
	return filteredCloneOf(source, remoteURL)
}

func filteredCloneOf(source string, remoteURL func(bare string) string, cloneFlags ...string) string {
	bare := filepath.Join(GinkgoT().TempDir(), "bare.git")
	gitOutputIn(source, "clone", "--bare", "--quiet", source, bare)
	gitOutputIn(bare, "config", "uploadpack.allowFilter", "true")
	clone := filepath.Join(GinkgoT().TempDir(), "part")
	arguments := append(append([]string{"clone", "--quiet"}, cloneFlags...), "--filter=blob:none", "file://"+bare, clone)
	gitOutputIn(GinkgoT().TempDir(), arguments...)
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

// skipUnlessGitCanRefuseLazyFetch skips a spec that rests on GIT_NO_LAZY_FETCH, which git has from 2.44.
func skipUnlessGitCanRefuseLazyFetch() {
	printed, err := exec.Command("git", "version").Output()
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	match := regexp.MustCompile(`^git version (\d+)\.(\d+)`).FindStringSubmatch(string(printed))
	ExpectWithOffset(1, match).NotTo(BeNil(), "git printed %q", printed)
	major, _ := strconv.Atoi(match[1])
	minor, _ := strconv.Atoi(match[2])
	if major < 2 || major == 2 && minor < 44 {
		Skip("this git (" + strings.TrimSpace(string(printed)) + ") predates GIT_NO_LAZY_FETCH, so it cannot be told not to fetch")
	}
}

// unreachableRemote is a remote whose transport is a program that could not answer, so a fetch that was
// attempted would show.
const unreachableRemote = "ssh://example.invalid/repo.git"

// scanArgs is a baseline scan that names the policy the partial-clone specs commit.
var scanArgs = []string{"codesignal", "--baseline", "--project-config", "cfg/p.json", "--format=json", "--no-interactive"}
