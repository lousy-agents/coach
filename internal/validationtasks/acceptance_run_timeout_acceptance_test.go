package validationtasks

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func fastAcceptanceTimeout(toml string) (int, bool) {
	return taskGoTestTimeout(toml, "test-acceptance-fast")
}

func distroGitRunTimeout(yml string) (int, bool) {
	return jobGoTestTimeout(yml, "distro-git")
}

func distroGitJobLimit(yml string) (int, bool) {
	return jobTimeoutMinutes(yml, "distro-git")
}

func mutated(text, old, replacement string) string {
	ExpectWithOffset(1, strings.Count(text, old)).To(Equal(1), "the mutant's anchor %q must occur once", old)
	return strings.Replace(text, old, replacement, 1)
}

var _ = Describe("the go test timeout of the other full-module runs (#909)", func() {
	const (
		floorMinutes = 25 // what `mise run test` uses (#605)
		distroRun    = "run: mise exec -- go run ./cmd/acceptance-ci-env -timeout 25m ./..."
		fastStep     = `"go test -race -timeout 25m ./... -run Acceptance",`
	)

	var toml, yml string

	BeforeEach(func() {
		raw, err := os.ReadFile(filepath.Join("..", "..", "mise.toml"))
		Expect(err).NotTo(HaveOccurred())
		toml = string(raw)
		raw, err = os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "ci.yml"))
		Expect(err).NotTo(HaveOccurred())
		yml = string(raw)
	})

	It("names a go test timeout on test-acceptance-fast, which runs cmd/coach under -race and would otherwise stop at go test's 10-minute default", func() {
		_, named := fastAcceptanceTimeout(toml)
		Expect(named).To(BeTrue())
	})

	It("gives test-acceptance-fast a go test timeout of at least as many minutes as `mise run test` uses", func() {
		minutes, _ := fastAcceptanceTimeout(toml)
		Expect(minutes).To(BeNumerically(">=", floorMinutes))
	})

	It("names a go test timeout on the distro-git leg's acceptance-ci-env run, which passes its arguments to go test", func() {
		_, named := distroGitRunTimeout(yml)
		Expect(named).To(BeTrue())
	})

	It("gives the distro-git leg a go test timeout of at least as many minutes as `mise run test` uses", func() {
		minutes, _ := distroGitRunTimeout(yml)
		Expect(minutes).To(BeNumerically(">=", floorMinutes))
	})

	It("leaves the distro-git job more time than its go test timeout, so go test panics with its goroutine dump before Actions cancels the job", func() {
		limit, _ := distroGitJobLimit(yml)
		timeout, _ := distroGitRunTimeout(yml)

		Expect(limit).To(BeNumerically(">=", timeout+5), "the other steps of the job need the margin")
	})

	DescribeTable("does not take a flag go test never receives for the distro-git leg's timeout",
		func(mutate func(string) string) {
			_, named := distroGitRunTimeout(mutate(yml))
			Expect(named).To(BeFalse())
		},
		Entry("a comment line in the job that mentions the flag, with the flag gone from the run line", func(yml string) string {
			yml = mutated(yml, distroRun, "run: mise exec -- go run ./cmd/acceptance-ci-env ./...")
			return mutated(yml, "      - name: Go tests under git 2.43", "      # acceptance-ci-env passes -timeout 25m through to go test (#909)\n      - name: Go tests under git 2.43")
		}),
		Entry("a trailing shell comment on the run line", func(yml string) string {
			return mutated(yml, distroRun, "run: mise exec -- go run ./cmd/acceptance-ci-env ./... # -timeout 25m")
		}),
		Entry("a test binary timeout after -args, which overrides go test's own", func(yml string) string {
			return mutated(yml, distroRun, distroRun+" -args -test.timeout 5m")
		}),
		Entry("a second command on the run line", func(yml string) string {
			return mutated(yml, distroRun, "run: mise exec -- go run ./cmd/acceptance-ci-env ./...; echo -timeout 25m")
		}),
	)

	DescribeTable("does not take a flag go test never receives for test-acceptance-fast's timeout",
		func(mutate func(string) string) {
			_, named := fastAcceptanceTimeout(mutate(toml))
			Expect(named).To(BeFalse())
		},
		Entry("a trailing shell comment inside the step", func(toml string) string {
			return mutated(toml, fastStep, `"go test -race ./... -run Acceptance # -timeout 25m",`)
		}),
		Entry("a test binary timeout after -args, which overrides go test's own", func(toml string) string {
			return mutated(toml, fastStep, `"go test -race -timeout 25m ./... -run Acceptance -args -test.timeout 5m",`)
		}),
		Entry("a second command in the step", func(toml string) string {
			return mutated(toml, fastStep, `"go test -race ./... -run Acceptance; echo -timeout 25m",`)
		}),
	)

	It("reads the distro-git job limit from the key, not from a comment that mentions it", func() {
		yml = mutated(yml, "    timeout-minutes: 60\n    env:\n      COACH_CI_GIT_VERSION: \"2.43.0\"", "    # timeout-minutes: 60\n    timeout-minutes: 28\n    env:\n      COACH_CI_GIT_VERSION: \"2.43.0\"")
		limit, _ := distroGitJobLimit(yml)

		Expect(limit).To(Equal(28))
	})

	It("keeps the sandbox recipe in validation-and-ci.md on the arguments the distro-git leg runs, so a contributor reproduces its timeout", func() {
		Expect(readRepoText("docs/development/validation-and-ci.md")).To(ContainSubstring("go run ./cmd/acceptance-ci-env -timeout 25m ./..."))
	})

	It("quotes test-acceptance-fast's go test command with its timeout in acceptance-harness.md", func() {
		Expect(readRepoText("docs/architecture/acceptance-harness.md")).To(ContainSubstring("go test -race -timeout 25m ./... -run Acceptance"))
	})

	It("quotes no run of the acceptance suites without a timeout in acceptance-harness.md", func() {
		Expect(readRepoText("docs/architecture/acceptance-harness.md")).NotTo(ContainSubstring("go test -race ./... -run Acceptance"))
	})
})
