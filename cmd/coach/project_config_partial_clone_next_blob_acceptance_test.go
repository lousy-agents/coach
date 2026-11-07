package main

import (
	"os"
	"os/exec"
	"regexp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal: every file a partial clone lacks is explained, not only the policy", func() {
	checkProject := func(clone string) (string, int) {
		_, stderr, exitCode := runCoachBinary(commandPath, clone, os.Environ(), "codesignal", "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "cfg/p.json", "--format", "json")
		return string(stderr), exitCode
	}

	paste := func(clone, command string) {
		run := exec.Command("sh", "-c", command)
		run.Dir = clone
		output, err := run.CombinedOutput()
		ExpectWithOffset(1, err).NotTo(HaveOccurred(), "the printed command must work: %s\n%s", command, output)
	}

	It("explains the next missing file after the policy's blob is fetched, and the printed batch fetch ends the failures", func() {
		skipUnlessGitCanRefuseLazyFetch()
		source := commitScannableGoRepo()
		commitFile(source, "cfg/p.json", singleRootPolicyJSON)
		commitFile(source, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
		clone := partialCloneOf(source, func(bare string) string { return "file://" + bare })

		stderr, exitCode := checkProject(clone)
		Expect(exitCode).To(Equal(1), "stderr:\n%s", stderr)
		paste(clone, regexp.MustCompile(`git cat-file -e '[^']*'`).FindString(stderr))

		stderr, exitCode = checkProject(clone)
		Expect(exitCode).To(Equal(1), "the policy is here now, and package.json is not:\n%s", stderr)
		Expect(stderr).To(ContainSubstring(`"package.json" could not be read at revision`), "stderr:\n%s", stderr)
		Expect(stderr).To(ContainSubstring("its content is not in this partial clone, and Coach does not fetch objects"), "stderr:\n%s", stderr)
		Expect(stderr).NotTo(ContainSubstring("exit status 1"), "a bare exit status tells the user nothing:\n%s", stderr)
		batch := printedBatch([]byte(stderr))
		Expect(batch).NotTo(BeEmpty(), "the message names the command that fetches the revision's files:\n%s", stderr)

		paste(clone, batch)
		stderr, exitCode = checkProject(clone)
		Expect(exitCode).To(Equal(0), "once the revision's files are here the check runs:\n%s", stderr)
	})
})
