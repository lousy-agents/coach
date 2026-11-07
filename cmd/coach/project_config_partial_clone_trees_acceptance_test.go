package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// treeLessCloneOf is a clone of source made with --filter=tree:0 and no checkout: it has the commits and none
// of their trees. The remote is set to the bare repository it was cloned from, which answers.
func treeLessCloneOf(source string) string {
	bare := filepath.Join(GinkgoT().TempDir(), "bare.git")
	gitOutputIn(source, "clone", "--bare", "--quiet", source, bare)
	gitOutputIn(bare, "config", "uploadpack.allowFilter", "true")
	clone := filepath.Join(GinkgoT().TempDir(), "treeless")
	gitOutputIn(GinkgoT().TempDir(), "clone", "--quiet", "--no-checkout", "--filter=tree:0", "file://"+bare, clone)
	return clone
}

// printedBatch is the command a refusal offers to fetch the revision's files, between the words that introduce
// it and "and run again" (with a comma when a single object is offered first).
func printedBatch(stderr []byte) string {
	match := regexp.MustCompile(`(?s)the revision's files with (for pass in .+?; done),? and run again`).FindSubmatch(stderr)
	if match == nil {
		return ""
	}
	return string(match[1])
}

func runPrinted(clone, command string) {
	run := exec.Command("sh", "-c", command)
	run.Dir = clone
	output, err := run.CombinedOutput()
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "the printed command must work: %s\n%s", command, output)
}

// A clone made without trees lacks the root tree of HEAD, and the first read of it, ls-tree, failed with git's
// "not a tree object" and none of the guidance a missing blob gets. Coach does not fetch, and says so: how many
// objects HEAD lacks, and the command that brings in what the scan reads. One paste of it is enough.
var _ = Describe("coach codesignal: a partial clone made without trees is refused with guidance, and one paste cures it", func() {
	var source string

	BeforeEach(func() {
		skipUnlessGitCanRefuseLazyFetch()
		source = commitScannableGoRepo()
		commitFile(source, "pkg/domain/d.go", minimalGoSource)
		commitFile(source, "cfg/p.json", singleRootPolicyJSON)
		commitFile(source, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
	})

	DescribeTable("a scan and a readiness check",
		func(args ...string) {
			clone := treeLessCloneOf(source)
			before := packFilesIn(clone)

			stdout, stderr, exitCode := runCoachBinary(commandPath, clone, os.Environ(), args...)

			Expect(exitCode).To(Equal(1), "stdout: %s\nstderr: %s", stdout, stderr)
			Expect(string(stderr)).To(ContainSubstring("this repository is a partial clone, and HEAD has 1 object that is not in it, so its files cannot be listed"), "stderr:\n%s", stderr)
			Expect(string(stderr)).To(ContainSubstring("Coach does not fetch objects"), "stderr:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("not a tree object"), "git's own words tell the user nothing:\n%s", stderr)
			Expect(string(stdout)).To(BeEmpty())
			Expect(packFilesIn(clone)).To(Equal(before), "the run wrote objects into the repository")
			batch := printedBatch(stderr)
			Expect(batch).NotTo(BeEmpty(), "the refusal names the command that fetches the revision's files:\n%s", stderr)

			runPrinted(clone, batch)
			stdout, stderr, exitCode = runCoachBinary(commandPath, clone, os.Environ(), args...)

			Expect(exitCode).To(Equal(0), "one paste of the printed command is enough: stdout: %s\nstderr: %s", stdout, stderr)
		},
		Entry("a baseline scan", "codesignal", "--baseline", "--format=json", "--no-interactive"),
		Entry("--check-project", "codesignal", "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "cfg/p.json", "--format", "json"),
	)

	It("cures a diff in one paste too: the batch lists HEAD as well as the merge base", func() {
		commitFile(source, "pkg/domain/d.go", minimalGoSource+"\nfunc changed() int { return 1 }\n")
		clone := treeLessCloneOf(source)
		args := []string{"codesignal", "--base", "HEAD~1", "--format=json", "--no-interactive"}

		_, stderr, exitCode := runCoachBinary(commandPath, clone, os.Environ(), args...)
		Expect(exitCode).To(Equal(1), "stderr:\n%s", stderr)
		batch := printedBatch(stderr)
		Expect(batch).NotTo(BeEmpty(), "stderr:\n%s", stderr)

		runPrinted(clone, batch)
		stdout, stderr, exitCode := runCoachBinary(commandPath, clone, os.Environ(), args...)

		Expect(exitCode).To(Equal(0), "stdout: %s\nstderr: %s", stdout, stderr)
		Expect(decodeCoachReport(stdout).Summary.FilesAnalyzed).To(Equal(1), "stdout:\n%s", stdout)
	})

	It("leaves a second paste of the batch harmless, when nothing is missing", func() {
		clone := treeLessCloneOf(source)
		_, stderr, _ := runCoachBinary(commandPath, clone, os.Environ(), "codesignal", "--baseline", "--format=json", "--no-interactive")
		batch := printedBatch(stderr)
		Expect(batch).NotTo(BeEmpty())
		runPrinted(clone, batch)
		before := packFilesIn(clone)

		runPrinted(clone, batch)

		Expect(packFilesIn(clone)).To(Equal(before), "a batch with nothing to fetch must not fetch")
	})
})
