package main

import (
	"os"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// changedFilePartialClone is a filtered clone of a repository whose second commit changes a.go, made with a
// checkout, so HEAD's blobs are local and the merge base's version of a.go, the one --base compares with,
// is not. The remote answers: the way out must be a command a user can run.
func changedFilePartialClone() string {
	source := commitScannableGoRepo()
	commitFile(source, "a.go", minimalGoSource+"\nfunc changed() int { return 1 }\n")
	return completePartialCloneOf(source, func(bare string) string { return "file://" + bare })
}

var diffArgs = []string{"codesignal", "--base", "HEAD~1", "--format=json", "--no-interactive"}

// A diff compares each changed file with its version at the merge base. In a partial clone that lacks it,
// the scan used to record the file as unreadable and exit 0 having analyzed none of the change, whatever
// --fail-on-incomplete-coverage said, which a CI job reads as a pass. It is the same refusal as for HEAD.
var _ = Describe("coach codesignal: a diff in a partial clone that lacks the merge base's files is refused", func() {
	DescribeTable("exits 1 with the objects it lacks and the command that brings them, and fetches nothing",
		func(extra ...string) {
			skipUnlessGitCanRefuseLazyFetch()
			clone := changedFilePartialClone()
			before := packFilesIn(clone)

			stdout, stderr, exitCode := runCoachBinary(commandPath, clone, os.Environ(), append(append([]string{}, diffArgs...), extra...)...)

			Expect(exitCode).To(Equal(1), "stdout: %s\nstderr: %s", stdout, stderr)
			Expect(string(stderr)).To(ContainSubstring("the merge base of HEAD~1 and HEAD has 1 object that is not in it"), "stderr:\n%s", stderr)
			Expect(string(stderr)).To(ContainSubstring("Coach does not fetch objects"), "stderr:\n%s", stderr)
			Expect(string(stderr)).To(ContainSubstring("git rev-list --objects --missing=print --no-walk"), "stderr:\n%s", stderr)
			Expect(string(stdout)).To(BeEmpty(), "a report was printed for a run that analyzed none of the change:\n%s", stdout)
			Expect(packFilesIn(clone)).To(Equal(before), "the run wrote objects into the repository")
		},
		Entry("a plain diff"),
		Entry("with --fail-on-incomplete-coverage", "--fail-on-incomplete-coverage"),
	)

	It("is cured by running the command it prints, after which the changed file is analyzed", func() {
		skipUnlessGitCanRefuseLazyFetch()
		clone := changedFilePartialClone()
		_, stderr, exitCode := runCoachBinary(commandPath, clone, os.Environ(), diffArgs...)
		Expect(exitCode).To(Equal(1), "stderr:\n%s", stderr)
		command := printedBatch(stderr)
		Expect(command).NotTo(BeEmpty(), "stderr names no command:\n%s", stderr)

		fetch := exec.Command("sh", "-c", command)
		fetch.Dir = clone
		fetch.Env = commitEnv
		output, err := fetch.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), "the printed command failed: %s\n%s", command, output)
		stdout, stderr, exitCode := runCoachBinary(commandPath, clone, os.Environ(), diffArgs...)

		Expect(exitCode).To(Equal(0), "stdout: %s\nstderr: %s", stdout, stderr)
		report := decodeCoachReport(stdout)
		Expect(report.Summary.FilesAnalyzed).To(Equal(1), "stdout:\n%s", stdout)
		Expect(report.Diagnostics).NotTo(ContainElement(HaveField("Kind", "base_read_failed")), "stdout:\n%s", stdout)
		Expect(report.Diagnostics).NotTo(ContainElement(HaveField("Kind", "diff_analysis_failed")), "stdout:\n%s", stdout)
	})

	It("still reports an unresolvable base in its own words", func() {
		skipUnlessGitCanRefuseLazyFetch()
		clone := changedFilePartialClone()

		_, stderr, exitCode := runCoachBinary(commandPath, clone, os.Environ(), "codesignal", "--base", "no-such-ref", "--format=json", "--no-interactive")

		Expect(exitCode).To(Equal(1), "stderr:\n%s", stderr)
		Expect(strings.ToLower(string(stderr))).To(ContainSubstring("cannot be resolved"), "stderr:\n%s", stderr)
	})
})
