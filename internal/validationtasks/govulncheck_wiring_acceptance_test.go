package validationtasks

import (
	"os"
	"path/filepath"
	"regexp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// govulncheckToolKey is the mise go-backend id. The aqua-registry recipe for
// this tool is type go_install, which mise's aqua backend rejects, and
// golang/vuln publishes no release assets, so the pin lives on this backend.
const govulncheckToolKey = "go:golang.org/x/vuln/cmd/govulncheck"

var govulncheckPinPattern = regexp.MustCompile(`(?m)^"` + regexp.QuoteMeta(govulncheckToolKey) + `"\s*=\s*"([^"]+)"\s*$`)

var exactGovulncheckVersion = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)

func vulncheckSteps(toml string) string {
	return taskSteps(toml, "vulncheck")
}

var _ = Describe("govulncheck wiring", func() {
	var toml, yml string

	BeforeEach(func() {
		rawTOML, err := os.ReadFile(filepath.Join("..", "..", "mise.toml"))
		Expect(err).NotTo(HaveOccurred())
		toml = string(rawTOML)

		rawYML, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "ci.yml"))
		Expect(err).NotTo(HaveOccurred())
		yml = string(rawYML)
	})

	// A floating "latest" would let a laptop and a runner install different
	// builds. The assignment has to be the tools table entry itself: a
	// comment that mentions the module path must not satisfy the pin.
	When("local runs and CI install govulncheck", func() {
		It("pins an exact module version in mise.toml", func() {
			m := govulncheckPinPattern.FindStringSubmatch(toml)
			Expect(m).NotTo(BeNil(),
				"mise.toml must pin %s so local and CI build the same govulncheck", govulncheckToolKey)
			Expect(m[1]).NotTo(Equal("latest"),
				"latest resolves again on each install; the pin has to be one release")
			Expect(m[1]).To(MatchRegexp(exactGovulncheckVersion.String()),
				"govulncheck version %q must be an exact vMAJOR.MINOR.PATCH module version, not a range", m[1])
		})
	})

	// Text output is the only format whose exit status is 3 when a vulnerable
	// symbol is reachable. -format json, sarif, and openvex exit 0 either way,
	// so a gate that switched formats would stay green with findings.
	// -scan symbol is that reachable-symbol check; package and module levels
	// fail on code the module never calls. -test and -tags are omitted: test
	// files and the thinproof/foreignclone/wasm configurations are not the
	// native module CI ships.
	When("the vulncheck task scans the module", func() {
		It("runs govulncheck in source mode at symbol precision over ./...", func() {
			steps := vulncheckSteps(toml)
			Expect(steps).NotTo(BeEmpty(),
				"without a vulncheck task, nothing local or in CI runs the scan")
			Expect(steps).To(ContainSubstring("govulncheck"),
				"the task has to invoke govulncheck")
			Expect(steps).To(ContainSubstring("-mode source"),
				"source mode is the module scan; binary and extract analyze one file")
			Expect(steps).To(ContainSubstring("-scan symbol"),
				"symbol precision fails only when a vulnerable symbol is reachable")
			Expect(steps).To(ContainSubstring("./..."),
				"the scan has to cover the module, not one package")
			Expect(steps).NotTo(ContainSubstring("-format"),
				"json, sarif, and openvex exit 0 when vulnerabilities are found, so they cannot be the gate")
		})

		It("does not analyze test files or an extra build-tag configuration", func() {
			steps := vulncheckSteps(toml)
			Expect(steps).NotTo(BeEmpty(), "vulncheck must exist for these exclusions to mean anything")
			Expect(steps).NotTo(ContainSubstring("-test"),
				"-test pulls test-only call stacks (ginkgo, thinproof, foreignclone) into a gate for code that does not ship")
			Expect(steps).NotTo(ContainSubstring("-tags"),
				"unix and linux files are already in the host GOOS build; thinproof, foreignclone, and the wasm grammar tags are other configurations")
		})
	})

	// ci-go is the verify job's step list. The scan is its own leaf, same as
	// wasm-build, so it belongs in ci-all and not in the verify mirror, the
	// per-cycle loop, or the seconds-long smoke check.
	When("choosing which local aggregates run the scan", func() {
		It("includes vulncheck in ci-all", func() {
			body := taskSteps(toml, "ci-all")
			Expect(body).NotTo(BeEmpty(), "ci-all must exist")
			Expect(body).To(ContainSubstring(`task = "vulncheck"`),
				"ci-all is the local run that reaches the leaves CI requires; a scan only a job runs is invisible locally")
		})

		It("keeps vulncheck out of ci-go, ci, ci-fast, and ci-gate", func() {
			for _, name := range []string{"ci-go", "ci", "ci-fast", "ci-gate"} {
				body := taskSteps(toml, name)
				Expect(body).NotTo(BeEmpty(), "%s must exist for this exclusion to mean anything", name)
				Expect(body).NotTo(ContainSubstring(`task = "vulncheck"`),
					"%s must not run vulncheck: ci-go mirrors verify, ci-fast is the edit loop, and ci-gate is the cheap smoke check", name)
			}
		})
	})

	When("CI gates on the scan", func() {
		It("runs vulncheck in its own job that installs Go and govulncheck", func() {
			job := jobBody(yml, "vulncheck")
			Expect(job).NotTo(BeEmpty(),
				"a task no job runs never executes on a pull request")
			Expect(job).To(ContainSubstring("mise run vulncheck"))
			Expect(job).To(MatchRegexp(`install_args:\s*"?go `+regexp.QuoteMeta(govulncheckToolKey)),
				"the job's install_args must name go and %s, not the full Node toolchain", govulncheckToolKey)
		})

		It("reuses the Go module cache", func() {
			job := jobBody(yml, "vulncheck")
			Expect(job).NotTo(BeEmpty(), "vulncheck job must exist")
			Expect(job).To(ContainSubstring("~/.cache/go-build"),
				"the scan loads every package; it should share the module build cache the other Go jobs use")
			Expect(job).To(ContainSubstring("~/go/pkg/mod"))
			Expect(job).To(ContainSubstring("hashFiles('go.sum')"))
		})

		It("is required by status and does not request code-scanning write", func() {
			job := jobBody(yml, "vulncheck")
			Expect(job).NotTo(BeEmpty(), "vulncheck job must exist")
			Expect(job).NotTo(ContainSubstring("security-events"),
				"SARIF upload needs security-events: write, and -format sarif would not fail the job when findings exist")
			Expect(job).NotTo(ContainSubstring("upload-sarif"))

			status := jobBody(yml, "status")
			Expect(status).NotTo(BeEmpty())
			Expect(needsList(status)).To(ContainElement("vulncheck"),
				"a leaf missing from status.needs can fail while the required check stays green")
		})
	})
})
