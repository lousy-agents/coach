package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// A scan that diffs a changed file makes git print a patch, and a patch is where git runs the programs a
// repository (or the environment) names for it: a textconv driver, picked by .gitattributes, converts each
// side before the diff, and an external diff program replaces the diff. Each channel has its own marker, so a
// failure says which program ran.
var _ = Describe("coach codesignal: a program git is configured to run on a patch is never run when a scan diffs a changed file", func() {
	// changedFileRepo is a repository whose last commit changes a.go, and the revision before it. Every
	// path in it is routed to the "evil" diff driver by an attribute.
	changedFileRepo := func() (repo, base string) {
		repo = commitExplicitPolicyRepo()
		base = strings.TrimSpace(gitIn(repo, "rev-parse", "HEAD"))
		Expect(os.WriteFile(filepath.Join(repo, ".gitattributes"), []byte("* diff=evil\n"), 0o644)).To(Succeed())
		gitIn(repo, "add", ".gitattributes")
		Expect(os.WriteFile(filepath.Join(repo, "a.go"), []byte(minimalGoSource+"// changed\n"), 0o644)).To(Succeed())
		gitIn(repo, "add", "a.go")
		commit := exec.Command("git", "commit", "-q", "-m", "change a.go")
		commit.Dir, commit.Env = repo, commitEnv
		Expect(commit.Run()).To(Succeed())
		return repo, base
	}
	markerProgram := func(marker string) string {
		path := filepath.Join(GinkgoT().TempDir(), "run.sh")
		Expect(os.WriteFile(path, []byte("#!/bin/sh\ntouch '"+marker+"'\ncat \"$@\"\n"), 0o755)).To(Succeed())
		return path
	}

	DescribeTable("runs no program the repository's configuration or the environment names",
		func(arrange func(repo, program string) []string) {
			marker := filepath.Join(GinkgoT().TempDir(), "ran")
			repo, base := changedFileRepo()
			env := append(os.Environ(), arrange(repo, markerProgram(marker))...)

			_, stderr, exitCode := runCoachBinary(commandPath, repo, env, "codesignal", "--base", base, "--format=json")

			Expect(exitCode).To(Equal(0), "the scan itself must still succeed: stderr: %s", stderr)
			Expect(marker).NotTo(BeAnExistingFile(), "git ran the program it was configured to run on a patch")
		},
		Entry("a textconv driver named in the repository's config", func(repo, program string) []string {
			gitIn(repo, "config", "diff.evil.textconv", program)
			return nil
		}),
		Entry("an external diff program named in the repository's config", func(repo, program string) []string {
			gitIn(repo, "config", "diff.external", program)
			return nil
		}),
		Entry("an external diff program named by GIT_EXTERNAL_DIFF", func(repo, program string) []string {
			return []string{"GIT_EXTERNAL_DIFF=" + program}
		}),
	)
})
