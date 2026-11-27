package validationtasks

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("the packages the distro-git legs run (#925, #920)", func() {
	var toml, yml string

	BeforeEach(func() {
		toml = readRepoText("mise.toml")
		yml = readRepoText(".github/workflows/ci.yml")
	})

	It("names, in distro-git's first command, the packages test-rest names, so the two lists cannot drift apart (D5)", func() {
		commands := distroCommands(yml, "distro-git")
		Expect(commands).NotTo(BeEmpty())

		Expect(packageArguments(commands[0])).To(Equal(packageArguments(restCommands(toml)[0])))
	})

	It("names every package of the module but cmd/coach, each exactly once, in distro-git's first command (D5)", func() {
		commands := distroCommands(yml, "distro-git")
		Expect(commands).NotTo(BeEmpty())
		listed := importPathsNamedBy(packageArguments(commands[0]))
		everything := goList("./...")

		Expect(listed).NotTo(HaveKey(coachPackage), "cmd/coach runs in distro-git's second command, with the skip")
		for _, path := range importPathsButCoach(everything) {
			Expect(listed).To(HaveKeyWithValue(path, 1), "%s runs once in distro-git", path)
		}
		Expect(listed).To(HaveLen(len(everything) - 1))
	})

	It("runs cmd/coach once in distro-git, in the command that skips the policy specs, and once in distro-git-policy (D5)", func() {
		Expect(commandsNaming(distroCommands(yml, "distro-git"), "./cmd/coach")).To(HaveLen(1))
		Expect(commandsNaming(distroCommands(yml, "distro-git-policy"), "./cmd/coach")).To(HaveLen(1))
	})
})
