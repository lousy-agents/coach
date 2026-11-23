package acceptancecienv

import (
	"os/exec"
	"regexp"
	"strings"

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

	It("#733 AC8: each of the four reports Skipped naming git 2.44 under an older git, and passes under 2.44 or later", func() {
		var quoted []string
		for _, text := range needs244 {
			quoted = append(quoted, regexp.QuoteMeta(text))
		}
		command := exec.Command("go", "test", "./cmd/coach", "-count=1", "-v", "-timeout=600s", "-run", "TestCoachAcceptance", "-args",
			"-ginkgo.v", "-ginkgo.no-color", "-ginkgo.focus="+strings.Join(quoted, "|"))
		command.Dir = repoRoot
		out, _ := command.CombinedOutput()
		output := string(out)

		Expect(output).To(MatchRegexp(`Will run [4-9] of`), "the focus must select the four specs; output:\n%s", output)
		if atLeast(hostGitVersion(), 2, 44) {
			Expect(output).To(ContainSubstring("SUCCESS!"), "output:\n%s", output)
			Expect(output).NotTo(ContainSubstring("[SKIPPED]"), "output:\n%s", output)
			return
		}
		Expect(output).NotTo(ContainSubstring("[FAILED]"), "under git %s they must skip, not fail; output:\n%s", hostGitVersion(), output)
		Expect(regexp.MustCompile(`\[SKIPPED\] needs git 2\.44`).FindAllString(output, -1)).To(HaveLen(len(needs244)), "each of the four must skip, naming the git it needs; output:\n%s", output)
	})
})
