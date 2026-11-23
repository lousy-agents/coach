package acceptancecienv

import (
	"os/exec"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func atLeast(version string, major, minor int) bool {
	parts := strings.SplitN(version, ".", 3)
	gotMajor, _ := strconv.Atoi(parts[0])
	gotMinor, _ := strconv.Atoi(parts[1])
	return gotMajor > major || gotMajor == major && gotMinor >= minor
}

var _ = Describe("a test that needs a newer git than the one running (#733 AC8)", func() {
	needs244 := []string{
		"TestOnAnOldGitABaseCommitOnlyTheRemoteHasIsRefusedWithoutRunningTheTransport",
		"TestTheStatusOfAGitThatCanBeToldNotToFetchSaysNothingWasSkipped",
	}

	for _, name := range needs244 {
		It("#733 AC8: "+name+" reports Skipped naming git 2.44 under an older git, and passes under 2.44 or later", func() {
			command := exec.Command("go", "test", "./internal/codesignalcli/gitrepo", "-count=1", "-v", "-run", "^"+name+"$")
			command.Dir = repoRoot
			out, _ := command.CombinedOutput()
			text := string(out)

			if atLeast(hostGitVersion(), 2, 44) {
				Expect(text).To(ContainSubstring("--- PASS: "+name), "output:\n%s", text)
				return
			}
			Expect(text).To(ContainSubstring("--- SKIP: "+name), "under git %s it must skip, not fail; output:\n%s", hostGitVersion(), text)
			Expect(text).To(ContainSubstring("2.44"), "the skip must name the git it needs; output:\n%s", text)
		})
	}
})
