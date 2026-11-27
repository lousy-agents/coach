package validationtasks

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func importPathsNamedBy(arguments []string) map[string]int {
	GinkgoHelper()
	listed := map[string]int{}
	for _, argument := range arguments {
		for _, path := range goList(argument) {
			listed[path]++
		}
	}
	return listed
}

func importPathsButCoach(everything []string) []string {
	var others []string
	for _, path := range everything {
		if path != coachPackage {
			others = append(others, path)
		}
	}
	return others
}

var _ = Describe("the packages of the rest half (#920, #605 Option B)", func() {
	var toml string

	BeforeEach(func() {
		toml = readRepoText("mise.toml")
	})

	It("names every package of the module but cmd/coach, each exactly once, in the rest half's first command (S3)", func() {
		arguments := packageArguments(restCommands(toml)[0])
		Expect(arguments).NotTo(BeEmpty())
		listed := importPathsNamedBy(arguments)
		everything := goList("./...")
		Expect(everything).To(ContainElement(coachPackage))

		Expect(listed).NotTo(HaveKey(coachPackage), "cmd/coach runs in the rest half's second command, with the skip")
		for _, path := range importPathsButCoach(everything) {
			Expect(listed).To(HaveKeyWithValue(path, 1), "%s runs once in the rest half", path)
		}
		Expect(listed).To(HaveLen(len(everything) - 1))
	})

	It("runs cmd/coach once in the rest half, in the command that skips the policy specs, and once in the policy half (S3)", func() {
		Expect(packageArguments(restCommands(toml)[1])).To(Equal([]string{"./cmd/coach"}))
		Expect(packageArguments(policyCommand(toml))).To(Equal([]string{"./cmd/coach"}))
	})
})
