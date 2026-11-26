package main_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"

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
				Expect(fixture.Bad.Result).To(Equal("not_covered"))
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
		before, _ := filepath.Glob(filepath.Join(os.TempDir(), "coach-eval-antipattern*"))
		writeFixture(fixtures, "layer", layerExpect, variantFiles(reportOf(layerSignal), nil), variantFiles(reportOf(), nil))

		outcome := runAntipatterns(fixtures, cannedCoach())

		Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
		after, _ := filepath.Glob(filepath.Join(os.TempDir(), "coach-eval-antipattern*"))
		Expect(after).To(HaveLen(len(before)))
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
		Entry("an expect.json whose id is not the directory's", `{"id":"other","status":"covered","expect":{"bad":"none","good":"none"}}`, variantFiles(reportOf(layerSignal), nil), "id"),
		Entry("a bad variant that fails go vet", layerExpect, variantFiles(reportOf(layerSignal), map[string]string{"main.go": "package main\n\nfunc main() { var unused int }\n"}), "go vet"),
		Entry("a bad variant with no go.mod", layerExpect, map[string]string{"main.go": validMain, "report.json": reportOf(layerSignal)}, "bad"),
	)

	It("passes every committed fixture against the real coach: Coach reports each anti-pattern on bad and stays silent on good", func() {
		root := filepath.Join(repositoryRoot(), "eval", "fixtures", "antipatterns")

		outcome := runAntipatterns(root, antipatternCoach())

		Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
		table := decodeTable(outcome.stdout)
		Expect(table.Summary.Fail).To(BeZero())
		covered := map[string]bool{}
		for _, fixture := range table.Fixtures {
			if fixture.Status == "covered" {
				covered[fixture.ID] = true
				Expect(fixture.Bad.Result).To(Equal("pass"), fixture.ID)
				Expect(fixture.Good.Result).To(Equal("pass"), fixture.ID)
			}
		}
		Expect(covered).To(HaveKey("dependency-rule-breach"))
		Expect(covered).To(HaveKey("skipped-layer"))
		Expect(covered).To(HaveKey("arrow-code"))
		Expect(covered).To(HaveKey("spaghetti-conditional"))
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
