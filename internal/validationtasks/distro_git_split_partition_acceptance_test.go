package validationtasks

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("the partition of cmd/coach's acceptance specs across the distro-git legs (#925, #920)", func() {
	var toml, yml string

	BeforeEach(func() {
		toml = readRepoText("mise.toml")
		yml = readRepoText(".github/workflows/ci.yml")
	})

	coachRest := func() string {
		commands := commandsNaming(distroCommands(yml, "distro-git"), "./cmd/coach")
		Expect(commands).To(HaveLen(1), "distro-git runs cmd/coach once, with the skip")
		return commands[0]
	}
	coachPolicy := func() string {
		commands := distroCommands(yml, "distro-git-policy")
		Expect(commands).To(HaveLen(1), "distro-git-policy runs one acceptance-ci-env command")
		return commands[0]
	}

	It("uses the pattern the verify legs use, once each, as the policy leg's focus-file and the rest leg's skip-file, so no spec runs in both distro legs or in neither (D1)", func() {
		verifyFocus := flagValues(policyCommand(toml), "ginkgo.focus-file")
		focus := flagValues(coachPolicy(), "ginkgo.focus-file")
		skip := flagValues(coachRest(), "ginkgo.skip-file")

		Expect(verifyFocus).To(HaveLen(1))
		Expect(focus).To(Equal(verifyFocus))
		Expect(skip).To(Equal(verifyFocus))
	})

	It("names the suite entry point with -run in the policy leg and in no command of the rest leg, so the plain Test functions of cmd/coach run once, in distro-git (D2)", func() {
		Expect(strings.Fields(coachPolicy())).To(ContainElements("-run", "TestCoachAcceptance"))
		for _, command := range distroCommands(yml, "distro-git") {
			Expect(strings.Fields(command)).NotTo(ContainElement("-run"), command)
		}
	})

	It("fails the policy leg when its pattern selects no spec, with -ginkgo.fail-on-empty (D3)", func() {
		Expect(strings.Fields(coachPolicy())).To(ContainElement("-ginkgo.fail-on-empty"))
	})

	It("runs two acceptance-ci-env commands in distro-git, the other packages and then cmd/coach, and one in distro-git-policy (D4)", func() {
		Expect(distroCommands(yml, "distro-git")).To(HaveLen(2))
		Expect(distroCommands(yml, "distro-git-policy")).To(HaveLen(1))
	})
})
