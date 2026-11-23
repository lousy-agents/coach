package acceptancecienv

import (
	"os/exec"
	"regexp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("a cmd/coach spec that needs GIT_NO_LAZY_FETCH, from git 2.44 (#733 AC8)", func() {
	needs244 := []string{
		"HEAD resolves to a commit whose tree object is missing from the object store",
		"HEAD resolves to a commit whose tree object is missing, and the repository path contains a character",
		"HEAD contains two Go modules and one module's go.mod blob is missing from the object store",
		"the policy is gone from disk and the object that holds its committed content is gone from the repository",
	}

	for _, text := range needs244 {
		It("#733 AC8: \""+text+"\" reports Skipped naming git 2.44 under an older git, and passes under 2.44 or later", func() {
			command := exec.Command("go", "test", "./cmd/coach", "-count=1", "-run", "TestCoachAcceptance", "-args",
				"-ginkgo.v", "-ginkgo.no-color", "-ginkgo.focus="+regexp.QuoteMeta(text))
			command.Dir = repoRoot
			out, _ := command.CombinedOutput()
			output := string(out)

			Expect(output).To(MatchRegexp(`Ran [1-9]\d* of`), "the focus must select the spec; output:\n%s", output)
			if atLeast(hostGitVersion(), 2, 44) {
				Expect(output).To(ContainSubstring("SUCCESS!"), "output:\n%s", output)
				Expect(output).NotTo(ContainSubstring("[SKIPPED]"), "output:\n%s", output)
				return
			}
			Expect(output).To(ContainSubstring("[SKIPPED]"), "under git %s it must skip, not fail; output:\n%s", hostGitVersion(), output)
			Expect(output).To(ContainSubstring("2.44"), "the skip must name the git it needs; output:\n%s", output)
			Expect(output).NotTo(ContainSubstring("[FAILED]"), "output:\n%s", output)
		})
	}
})
