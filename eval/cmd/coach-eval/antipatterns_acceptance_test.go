package main_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach-eval antipatterns (#871)", func() {
	var fixtures string

	BeforeEach(func() { fixtures = GinkgoT().TempDir() })

	It("passes a fixture whose bad variant has the expected finding and whose good variant has none, and prints the table", func() {
		writeFixture(fixtures, "layer", layerExpect, variantFiles(reportOf(layerSignal), nil), variantFiles(reportOf(), nil))

		outcome := runAntipatterns(fixtures, cannedCoach())

		Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
		table := decodeTable(outcome.stdout)
		Expect(table.Fixtures).To(HaveLen(1))
		Expect(table.Fixtures[0].ID).To(Equal("layer"))
		Expect(table.Fixtures[0].Bad.Result).To(Equal("pass"))
		Expect(table.Fixtures[0].Good.Result).To(Equal("pass"))
		Expect(table.Summary.Pass).To(Equal(1))
	})

	DescribeTable("exits 1 and names the fixture, the variant and the rule when a covered fixture fails",
		func(bad, good string, variant string) {
			writeFixture(fixtures, "layer", layerExpect, variantFiles(bad, nil), variantFiles(good, nil))

			outcome := runAntipatterns(fixtures, cannedCoach())

			Expect(outcome.exitCode).To(Equal(1), outcome.stderr)
			Expect(outcome.stderr).To(ContainSubstring("layer"))
			Expect(outcome.stderr).To(ContainSubstring(variant))
			Expect(outcome.stderr).To(ContainSubstring("architecture.layer_violation"))
			Expect(decodeTable(outcome.stdout).Summary.Fail).To(Equal(1))
		},
		Entry("the bad variant is silent", reportOf(), reportOf(), "bad"),
		Entry("the good variant reports the rule", reportOf(layerSignal), reportOf(layerSignal), "good"),
		Entry("the bad variant reports the rule at another path", reportOf("architecture.layer_violation|internal/domain/other.go|architecture.layer_violation:internal/domain->internal/infra"), reportOf(), "bad"),
		Entry("the bad variant reports it with another subject", reportOf("architecture.layer_violation|internal/domain/order.go|architecture.layer_violation:internal/domain->internal/other"), reportOf(), "bad"),
	)

	It("exits 1 when the bad variant also reports a rule nobody expected", func() {
		extra := "complexity.branch_density|internal/domain/order.go|complexity.branch_density:order"
		writeFixture(fixtures, "layer", layerExpect, variantFiles(reportOf(layerSignal, extra), nil), variantFiles(reportOf(), nil))

		outcome := runAntipatterns(fixtures, cannedCoach())

		Expect(outcome.exitCode).To(Equal(1), outcome.stderr)
		Expect(outcome.stderr).To(ContainSubstring("complexity.branch_density"))
	})

	It("exits 1 when the good variant reports a rule nobody expected", func() {
		extra := "complexity.branch_density|internal/domain/order.go|complexity.branch_density:order"
		writeFixture(fixtures, "layer", layerExpect, variantFiles(reportOf(layerSignal), nil), variantFiles(reportOf(extra), nil))

		outcome := runAntipatterns(fixtures, cannedCoach())

		Expect(outcome.exitCode).To(Equal(1), outcome.stderr)
		Expect(outcome.stderr).To(ContainSubstring("good"))
	})

	const missExpect = `{"id":"global","name":"Global mutable state","source":"none","seed":"none","status":"not_covered",
 "expect":{"bad":"none","good":"none"}}`

	It("reports a known miss as not_covered, lists it in the summary, and does not fail the run for it", func() {
		writeFixture(fixtures, "layer", layerExpect, variantFiles(reportOf(layerSignal), nil), variantFiles(reportOf(), nil))
		writeFixture(fixtures, "global", missExpect, variantFiles(reportOf(), nil), variantFiles(reportOf(), nil))

		outcome := runAntipatterns(fixtures, cannedCoach())

		Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
		table := decodeTable(outcome.stdout)
		Expect(table.Summary.NotCovered).To(Equal([]string{"global"}))
		for _, fixture := range table.Fixtures {
			if fixture.ID == "global" {
				Expect(fixture.Status).To(Equal("not_covered"))
				Expect(fixture.Bad.Result).To(Equal("miss"))
			}
		}
		Expect(table.Summary.Pass).To(Equal(1), "a known miss is never counted as a pass")
		Expect(outcome.stderr).NotTo(ContainSubstring("FAIL"), "a known miss is listed, not reported as a failure")
	})

	It("ignores a file that sits beside the fixtures, such as a README", func() {
		writeFixture(fixtures, "layer", layerExpect, variantFiles(reportOf(layerSignal), nil), variantFiles(reportOf(), nil))
		Expect(os.WriteFile(filepath.Join(fixtures, "README.md"), []byte("fixtures"), 0o644)).To(Succeed())

		outcome := runAntipatterns(fixtures, cannedCoach())

		Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
	})

	It("says so, without failing, when Coach starts to report on a known miss", func() {
		writeFixture(fixtures, "global", missExpect,
			variantFiles(reportOf("state.hidden_input_mutation|main.go|state.hidden_input_mutation:x"), nil), variantFiles(reportOf(), nil))

		outcome := runAntipatterns(fixtures, cannedCoach())

		Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
		Expect(decodeTable(outcome.stdout).Fixtures[0].Bad.Result).To(Equal("now_covered"))
	})

	It("reports, for every variant, whether coverage was complete and which diagnostics the scan raised", func() {
		writeFixture(fixtures, "layer", layerExpect, variantFiles(reportOf(layerSignal), nil), variantFiles(reportOf(), nil))

		outcome := runAntipatterns(fixtures, cannedCoach())

		Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
		fixture := decodeTable(outcome.stdout).Fixtures[0]
		Expect(fixture.Bad.CoverageComplete).To(BeTrue())
		Expect(fixture.Good.CoverageComplete).To(BeTrue())
		Expect(fixture.Bad.Diagnostics).To(BeEmpty())
	})

	DescribeTable("exits 1 and says the scan was incomplete when a variant's coverage was not complete and the fixture did not say so",
		func(bad, good map[string]string, variant string) {
			writeFixture(fixtures, "layer", layerExpect, bad, good)

			outcome := runAntipatterns(fixtures, cannedCoach())

			Expect(outcome.exitCode).To(Equal(1), outcome.stderr)
			Expect(outcome.stderr).To(ContainSubstring("layer " + variant))
			Expect(outcome.stderr).To(ContainSubstring("incomplete"))
			Expect(decodeTable(outcome.stdout).Fixtures[0]).NotTo(BeNil())
		},
		Entry("a silent good variant on an incomplete scan is not a pass",
			variantFiles(reportOf(layerSignal), nil), incompleteVariant(reportOf()), "good"),
		Entry("a bad variant that has its finding but whose scan was incomplete is not a pass",
			incompleteVariant(reportOf(layerSignal)), variantFiles(reportOf(), nil), "bad"),
	)

	It("marks the variant's result incomplete, with its diagnostics, rather than pass", func() {
		writeFixture(fixtures, "layer", layerExpect, variantFiles(reportOf(layerSignal), nil), incompleteVariant(reportOf()))

		outcome := runAntipatterns(fixtures, cannedCoach())

		good := decodeTable(outcome.stdout).Fixtures[0].Good
		Expect(good.Result).To(Equal("incomplete"))
		Expect(good.CoverageComplete).To(BeFalse())
		Expect(good.Diagnostics).To(ConsistOf("project_coverage_incomplete"))
		Expect(decodeTable(outcome.stdout).Summary.Incomplete).To(Equal([]string{"layer"}), "the summary lists a fixture whose good variant was incomplete too")
	})

	const incompleteMissExpect = `{"id":"global","name":"Bypass","source":"none","seed":"none","status":"not_covered","expect_incomplete":true,
 "expect":{"bad":"none","good":"none"}}`

	It("accepts an incomplete scan that the fixture declares, and records it as incomplete rather than as a miss", func() {
		writeFixture(fixtures, "global", incompleteMissExpect, incompleteVariant(reportOf()), variantFiles(reportOf(), nil))

		outcome := runAntipatterns(fixtures, cannedCoach())

		Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
		table := decodeTable(outcome.stdout)
		Expect(table.Fixtures[0].Bad.Result).To(Equal("incomplete"))
		Expect(table.Fixtures[0].Good.Result).To(Equal("miss"))
		Expect(table.Summary.Incomplete).To(Equal([]string{"global"}))
	})

	It("exits 1 when a fixture declares an incomplete scan and the scan was complete", func() {
		writeFixture(fixtures, "global", incompleteMissExpect, variantFiles(reportOf(), nil), variantFiles(reportOf(), nil))

		outcome := runAntipatterns(fixtures, cannedCoach())

		Expect(outcome.exitCode).To(Equal(1), outcome.stderr)
		Expect(outcome.stderr).To(ContainSubstring("global bad"))
		Expect(outcome.stderr).To(ContainSubstring("complete"))
	})

	It("exits 1 for an unexpectedly incomplete scan of a known miss, which is not a miss", func() {
		writeFixture(fixtures, "global", missExpect, incompleteVariant(reportOf()), variantFiles(reportOf(), nil))

		outcome := runAntipatterns(fixtures, cannedCoach())

		Expect(outcome.exitCode).To(Equal(1), outcome.stderr)
		Expect(outcome.stderr).To(ContainSubstring("global bad"))
		Expect(decodeTable(outcome.stdout).Fixtures[0].Bad.Result).To(Equal("incomplete"))
	})

	It("exits 2 when coach fails with a status that is neither complete nor incomplete", func() {
		writeFixture(fixtures, "layer", layerExpect,
			variantFiles(reportOf(layerSignal), map[string]string{"exit_code": "1"}), variantFiles(reportOf(), nil))

		outcome := runAntipatterns(fixtures, cannedCoach())

		Expect(outcome.exitCode).To(Equal(2), outcome.stderr)
		Expect(outcome.stdout).To(BeEmpty())
		Expect(outcome.stderr).To(ContainSubstring("layer"))
	})

	It("scans each variant with --baseline, and with --project-config only when the variant has a policy", func() {
		withPolicy := variantFiles(reportOf(layerSignal), map[string]string{".lousy-agents/coach.json": `{"schema_version":"1","roots":["."]}`})
		writeFixture(fixtures, "layer", layerExpect, withPolicy, variantFiles(reportOf(), nil))

		outcome := runAntipatterns(fixtures, cannedCoach())

		Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
		table := decodeTable(outcome.stdout)
		Expect(table.Fixtures[0].Bad.Scan).To(ContainSubstring("--baseline"))
		Expect(table.Fixtures[0].Bad.Scan).To(ContainSubstring("--project-config .lousy-agents/coach.json"))
		Expect(table.Fixtures[0].Good.Scan).To(ContainSubstring("--baseline"))
		Expect(table.Fixtures[0].Good.Scan).NotTo(ContainSubstring("--project-config"))
	})

	It("reads a variant stored with .txt suffixes on its sources, so the repository's own scan never sees it", func() {
		bad := map[string]string{"go.mod.txt": validMod, "main.go.txt": validMain, "report.json": reportOf(layerSignal)}
		good := map[string]string{"go.mod.txt": validMod, "main.go.txt": validMain, "report.json": reportOf()}
		writeFixture(fixtures, "layer", layerExpect, bad, good)

		outcome := runAntipatterns(fixtures, cannedCoach())

		Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
	})

	It("reads a policy stored as coach.json.txt as the variant's policy", func() {
		bad := map[string]string{"go.mod.txt": validMod, "main.go.txt": validMain, "report.json": reportOf(layerSignal),
			".lousy-agents/coach.json.txt": `{"schema_version":"1","roots":["."]}`}
		good := map[string]string{"go.mod.txt": validMod, "main.go.txt": validMain, "report.json": reportOf()}
		writeFixture(fixtures, "layer", layerExpect, bad, good)

		outcome := runAntipatterns(fixtures, cannedCoach())

		Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
		Expect(decodeTable(outcome.stdout).Fixtures[0].Bad.Scan).To(ContainSubstring("--project-config .lousy-agents/coach.json"))
	})

	It("leaves nothing of a scanned variant behind", func() {
		scratch := GinkgoT().TempDir()
		writeFixture(fixtures, "layer", layerExpect, variantFiles(reportOf(layerSignal), nil), variantFiles(reportOf(), nil))

		outcome := runAntipatternsWith([]string{"TMPDIR=" + scratch}, fixtures, cannedCoach())

		Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
		left, err := os.ReadDir(scratch)
		Expect(err).NotTo(HaveOccurred())
		Expect(left).To(BeEmpty(), "every temporary directory of the run is removed")
	})

	It("commits a variant without the host's git configuration, so a host hook cannot fail the scan", func() {
		hooks := GinkgoT().TempDir()
		Expect(os.WriteFile(filepath.Join(hooks, "pre-commit"), []byte("#!/bin/sh\nexit 1\n"), 0o755)).To(Succeed())
		home := GinkgoT().TempDir()
		Expect(os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("[core]\n\thooksPath = "+hooks+"\n"), 0o644)).To(Succeed())
		writeFixture(fixtures, "layer", layerExpect, variantFiles(reportOf(layerSignal), nil), variantFiles(reportOf(), nil))

		outcome := runAntipatternsWith([]string{"HOME=" + home}, fixtures, cannedCoach())

		Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
	})

	It("scans a variant in its own repository when the caller's environment names another, as in a pre-commit hook", func() {
		other := GinkgoT().TempDir()
		Expect(exec.Command("git", "init", "-q", other).Run()).To(Succeed())
		writeFixture(fixtures, "layer", layerExpect, variantFiles(reportOf(layerSignal), nil), variantFiles(reportOf(), nil))

		outcome := runAntipatternsWith([]string{
			"GIT_DIR=" + filepath.Join(other, ".git"), "GIT_WORK_TREE=" + other, "GIT_INDEX_FILE=" + filepath.Join(other, ".git", "index"),
		}, fixtures, cannedCoach())

		Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
		count, err := exec.Command("git", "-C", other, "rev-list", "--all", "--count").Output()
		Expect(err).NotTo(HaveOccurred())
		Expect(strings.TrimSpace(string(count))).To(Equal("0"), "the caller's repository gained no commit")
	})

	It("runs a coach named by a relative path, from the directory the command was started in", func() {
		writeFixture(fixtures, "layer", layerExpect, variantFiles(reportOf(layerSignal), nil), variantFiles(reportOf(), nil))
		coach := cannedCoach()
		cmd := exec.Command(coachEvalBinary, "antipatterns", "--fixtures", fixtures, "--coach", "./"+filepath.Base(coach))
		cmd.Dir = filepath.Dir(coach)
		cmd.Env = os.Environ()

		outcome := runGenerateCommand(cmd)

		Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
	})

	It("exits 2 when a coach report has no signals key, rather than reading it as no findings", func() {
		writeFixture(fixtures, "layer", layerExpect, variantFiles(`{"diagnostics":[]}`, nil), variantFiles(reportOf(), nil))

		outcome := runAntipatterns(fixtures, cannedCoach())

		Expect(outcome.exitCode).To(Equal(2), outcome.stderr)
		Expect(outcome.stdout).To(BeEmpty())
		Expect(outcome.stderr).To(ContainSubstring("layer"))
		Expect(outcome.stderr).To(ContainSubstring("signals"))
	})

	DescribeTable("exits 2 and names the fixture when its input is unusable",
		func(expect string, bad map[string]string, mentions string) {
			writeFixture(fixtures, "layer", expect, bad, variantFiles(reportOf(), nil))

			outcome := runAntipatterns(fixtures, cannedCoach())

			Expect(outcome.exitCode).To(Equal(2), outcome.stderr)
			Expect(outcome.stdout).To(BeEmpty())
			Expect(outcome.stderr).To(ContainSubstring("layer"))
			Expect(outcome.stderr).To(ContainSubstring(mentions))
		},
		Entry("no expect.json", "", variantFiles(reportOf(layerSignal), nil), "expect.json"),
		Entry("an expect.json that is not JSON", "{", variantFiles(reportOf(layerSignal), nil), "expect.json"),
		Entry("an expect.json with an unknown status", `{"id":"layer","status":"maybe","expect":{"bad":"none","good":"none"}}`, variantFiles(reportOf(layerSignal), nil), "status"),
		Entry("a covered fixture that expects nothing on its bad variant", `{"id":"layer","status":"covered","expect":{"bad":"none","good":"none"}}`, variantFiles(reportOf(), nil), "bad"),
		Entry("an expect.json with a key it does not know", `{"id":"layer","status":"covered","expcet":{},"expect":{"bad":[{"rule_id":"r","path":"p","subject":"s"}],"good":"none"}}`, variantFiles(reportOf(layerSignal), nil), "expcet"),
		Entry("an expect.json whose id is not the directory's", `{"id":"other","status":"covered","expect":{"bad":"none","good":"none"}}`, variantFiles(reportOf(layerSignal), nil), "id"),
		Entry("a bad variant that fails go vet", layerExpect, variantFiles(reportOf(layerSignal), map[string]string{"main.go": "package main\n\nfunc main() { var unused int }\n"}), "go vet"),
		Entry("a bad variant with no go.mod", layerExpect, map[string]string{"main.go": validMain, "report.json": reportOf(layerSignal)}, "bad"),
	)

	It("records, for the real coach, the unmatched-required-layer fixture as an incomplete scan with the diagnostic that says so", func() {
		root := filepath.Join(repositoryRoot(), "eval", "fixtures", "antipatterns")

		outcome := runAntipatterns(root, antipatternCoach())

		Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
		for _, fixture := range decodeTable(outcome.stdout).Fixtures {
			if fixture.ID == "bypass-without-required-layer" {
				Expect(fixture.Bad.Result).To(Equal("incomplete"))
				Expect(fixture.Bad.Diagnostics).To(ContainElement("project_layer_bypass_coverage_incomplete"))
				Expect(fixture.Good.CoverageComplete).To(BeTrue())
				continue
			}
			Expect(fixture.Bad.CoverageComplete).To(BeTrue(), fixture.ID)
			Expect(fixture.Good.CoverageComplete).To(BeTrue(), fixture.ID)
		}
	})

	It("passes every committed fixture against the real coach: Coach reports each anti-pattern on bad and stays silent on good", func() {
		root := filepath.Join(repositoryRoot(), "eval", "fixtures", "antipatterns")

		outcome := runAntipatterns(root, antipatternCoach())

		Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
		table := decodeTable(outcome.stdout)
		Expect(table.Summary.Fail).To(BeZero())
		covered := coveredFixtureIDs(table)
		Expect(covered).To(HaveKey("dependency-rule-breach"))
		Expect(covered).To(HaveKey("skipped-layer"))
		Expect(covered).To(HaveKey("arrow-code"))
		Expect(covered).To(HaveKey("spaghetti-conditional"))
		Expect(covered).To(HaveKey("god-file"))
		Expect(covered).To(HaveKey("hidden-input-mutation"))
		Expect(covered).To(HaveKey("check-then-act"))
	})

	It("records, in the expect.json of the arrow-code fixture, that a deeper arrow also trips cognitive complexity", func() {
		content, err := os.ReadFile(filepath.Join(repositoryRoot(), "eval", "fixtures", "antipatterns", "arrow-code", "expect.json"))
		Expect(err).NotTo(HaveOccurred())

		var expect struct {
			Note string `json:"note"`
		}
		Expect(json.Unmarshal(content, &expect)).To(Succeed())
		Expect(expect.Note).To(ContainSubstring("complexity.cognitive_complexity"))
		Expect(expect.Note).To(ContainSubstring("not exhaustive"))
	})

	It("records, in the expect.json of the bypass-without-required-layer fixture, what the scan says about the silence", func() {
		content, err := os.ReadFile(filepath.Join(repositoryRoot(), "eval", "fixtures", "antipatterns", "bypass-without-required-layer", "expect.json"))
		Expect(err).NotTo(HaveOccurred())

		var expect struct {
			Note string `json:"note"`
		}
		Expect(json.Unmarshal(content, &expect)).To(Succeed())
		Expect(expect.Note).To(ContainSubstring("project_layer_bypass_coverage_incomplete"))
		Expect(expect.Note).To(ContainSubstring("--fail-on-incomplete-coverage"))
	})

	It("lists the anti-patterns Coach has no rule for as not_covered, and counts none of them as a pass", func() {
		root := filepath.Join(repositoryRoot(), "eval", "fixtures", "antipatterns")

		outcome := runAntipatterns(root, antipatternCoach())

		Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
		table := decodeTable(outcome.stdout)
		Expect(table.Summary.NotCovered).To(ConsistOf("bypass-without-required-layer", "constructor-does-real-work", "create-after-missing-check", "global-mutable-state", "god-struct"))
		expectKnownMissesSilent(table)
		Expect(table.Summary.Pass).To(Equal(7), "seven covered fixtures pass; the known misses are not among them")
	})

	It("carries no finding of its own: Coach, scanning the committed fixture tree as a repository, reports nothing", func() {
		repo := GinkgoT().TempDir()
		copyDirectory(filepath.Join(repositoryRoot(), "eval", "fixtures", "antipatterns"), repo)
		for _, step := range [][]string{{"init", "-q"}, {"add", "-A"}, {"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "x"}} {
			cmd := exec.Command("git", step...)
			cmd.Dir = repo
			Expect(cmd.Run()).To(Succeed())
		}
		scan := exec.Command(antipatternCoach(), "codesignal", "--baseline", "--no-interactive", "--format", "json")
		scan.Dir = repo

		outcome := runGenerateCommand(scan)

		Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
		var report struct{ Signals []map[string]any }
		Expect(json.Unmarshal([]byte(outcome.stdout), &report)).To(Succeed(), outcome.stdout)
		Expect(report.Signals).To(BeEmpty(), "the fixtures are stored as .txt, so the repository's own scan sees none of them")
	})

	DescribeTable("exits 2 for a missing tool or fixture directory",
		func(args func() (string, string), mentions string) {
			dir, coach := args()

			outcome := runAntipatterns(dir, coach)

			Expect(outcome.exitCode).To(Equal(2), outcome.stderr)
			Expect(outcome.stdout).To(BeEmpty())
			Expect(outcome.stderr).To(ContainSubstring(mentions))
		},
		Entry("no --coach", func() (string, string) { return GinkgoT().TempDir(), "" }, "--coach"),
		Entry("a coach that does not exist", func() (string, string) { return GinkgoT().TempDir(), "/nonexistent/coach" }, "/nonexistent/coach"),
		Entry("a fixtures directory that does not exist", func() (string, string) { return "/nonexistent/fixtures", cannedCoach() }, "/nonexistent/fixtures"),
		Entry("a fixtures directory with no fixture", func() (string, string) { return GinkgoT().TempDir(), cannedCoach() }, "no fixture"),
	)
})
