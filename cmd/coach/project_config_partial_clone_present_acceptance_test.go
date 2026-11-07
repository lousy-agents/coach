package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The policy's blob is missing from a partial clone whether or not the file is on disk, and the user is told so
// in the same sentence: which question went unanswered differs, the way to fetch the object does not.
var _ = Describe("coach codesignal --check-project: a policy blob a partial clone lacks is worded once", func() {
	notLocalSentence := regexp.MustCompile(`its content is not in [^\n]*`)

	checkProjectFailure := func(clone string) string {
		stdout, stderr, exitCode := runCoachBinary(commandPath, clone, os.Environ(), "codesignal", "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "cfg/p.json", "--format", "json")
		Expect(exitCode).To(Equal(1), "stdout: %s\nstderr: %s", stdout, stderr)
		return string(stderr)
	}

	It("says the same thing when the file is on disk as when it is not", func() {
		skipUnlessGitCanRefuseLazyFetch()
		absent := commitPartialClone(func(string) string { return unreachableRemote })
		present := commitPartialClone(func(string) string { return unreachableRemote })
		putPolicyOnDisk(present)

		whenAbsent := checkProjectFailure(absent)
		whenPresent := checkProjectFailure(present)

		Expect(notLocalSentence.FindString(whenPresent)).To(HavePrefix("its content is not in this partial clone, and Coach does not fetch objects; fetch it with git cat-file -e"), "stderr:\n%s", whenPresent)
		Expect(notLocalSentence.FindString(whenPresent)).To(Equal(strings.ReplaceAll(notLocalSentence.FindString(whenAbsent), strings.TrimSpace(gitOutputIn(absent, "rev-parse", "HEAD")), strings.TrimSpace(gitOutputIn(present, "rev-parse", "HEAD")))), "the two states word the missing blob differently:\nabsent:  %s\npresent: %s", whenAbsent, whenPresent)
		Expect(whenPresent).NotTo(ContainSubstring("local object store"), "stderr:\n%s", whenPresent)
	})

	It("names the command the user ran and no other", func() {
		skipUnlessGitCanRefuseLazyFetch()
		clone := commitPartialClone(func(string) string { return unreachableRemote })
		putPolicyOnDisk(clone)

		stderr := checkProjectFailure(clone)

		Expect(stderr).To(HavePrefix("coach codesignal --check-project: "), "stderr:\n%s", stderr)
	})
})

func putPolicyOnDisk(clone string) {
	ExpectWithOffset(1, os.MkdirAll(filepath.Join(clone, "cfg"), 0o755)).To(Succeed())
	ExpectWithOffset(1, os.WriteFile(filepath.Join(clone, "cfg", "p.json"), []byte(singleRootPolicyJSON), 0o644)).To(Succeed())
}
