package evalharness_test

import (
	"maps"
	"path"
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
	"github.com/lousy-agents/coach/eval/internal/evalharness"
)

var firstTestFunction = regexp.MustCompile(`func (Test\w+)\(t \*testing\.T\) \{`)

func cloned(generated evalgen.Program) evalgen.Program {
	generated.Solution = maps.Clone(generated.Solution)
	generated.Tests = maps.Clone(generated.Tests)
	generated.FollowOnSolution = maps.Clone(generated.FollowOnSolution)
	generated.FollowOnTests = maps.Clone(generated.FollowOnTests)
	generated.Manifest.Template.VisibleTests = append([]evalgen.VisibleTest(nil), generated.Manifest.Template.VisibleTests...)
	return generated
}

func anyKey(files map[string][]byte) string {
	for name := range files {
		return name
	}
	Fail("no file to anchor on")
	return ""
}

// withSkippedTest returns a copy of files in which the first test function of the named file skips, and that
// function's name.
func withSkippedTest(files map[string][]byte, name string) (map[string][]byte, string) {
	source := string(files[name])
	match := firstTestFunction.FindStringSubmatch(source)
	Expect(match).NotTo(BeNil(), "a test function in %s", name)
	skipped := maps.Clone(files)
	skipped[name] = []byte(strings.Replace(source, match[0], match[0]+"\n\tt.Skip(\"not today\")", 1))
	return skipped, match[1]
}

// withHiddenTest returns a copy of the suite with a passing test added to the package of its first file.
func withHiddenTest(files map[string][]byte) map[string][]byte {
	name := anyKey(files)
	packageClause := regexp.MustCompile(`(?m)^package \w+`).FindString(string(files[name]))
	extended := maps.Clone(files)
	extended[path.Dir(name)+"/extra_test.go"] = []byte(packageClause + "\n\nimport \"testing\"\n\nfunc TestExtraHidden(t *testing.T) {}\n")
	return extended
}

func testCount(files map[string][]byte) int {
	total := 0
	for _, content := range files {
		total += len(firstTestFunction.FindAllString(string(content), -1))
	}
	return total
}

func visibleCount(generated evalgen.Program) int {
	total := 0
	for _, visible := range generated.Manifest.Template.VisibleTests {
		total += len(visible.TestIDs)
	}
	return total
}

var _ = Describe("the oracles' G2 and native G1 checks (#780; design §6 and §8)", func() {
	DescribeTable("accept the oracles of a generated program, and count the tests each oracle ran (AC-28)",
		func(cell int) {
			generated := program(cell)

			grades, err := evalharness.GradeOracles(generated)

			Expect(err).NotTo(HaveOccurred())
			Expect(grades.FirstOracleTests).To(Equal(visibleCount(generated)+testCount(generated.Tests)), "the visible tests and the hidden feature tests")
			Expect(grades.FollowOnOracleTests).To(Equal(visibleCount(generated)+testCount(generated.FollowOnTests)), "the visible tests and the follow-on tests, which replace the feature tests")
		},
		Entry("the first cell", 0),
		Entry("a cell with an http handler entry", 1002),
		Entry("a write cell", 1500),
	)

	Describe("G2", func() {
		grading := `package %s

import "os"

var Descriptor = os.NewFile(3, "grading")
`

		leak := func(generated evalgen.Program) (name string, content []byte) {
			directory := writeBase(generated)
			first, last := firstAndLastLayerDirectories(directory)
			return first + "/leak.go", []byte("package " + path.Base(first) + "\n\nimport _ \"" + moduleOf(generated) + "/" + last + "\"\n")
		}

		DescribeTable("rejects at the g2 step, naming the rule, when either oracle breaks it (AC-28)",
			func(rule string, mutate func(generated *evalgen.Program)) {
				generated := cloned(program(0))
				mutate(&generated)

				_, err := evalharness.GradeOracles(generated)

				Expect(err).To(HaveOccurred())
				Expect(rejectionStep(err)).To(Equal(evalgen.StepG2))
				Expect(err.Error()).To(ContainSubstring(rule))
			},
			Entry("grading-channel access in the first oracle's solution", "gaming.grading_channel_access", func(generated *evalgen.Program) {
				directory := path.Dir(anyKey(generated.Solution))
				generated.Solution[directory+"/channel.go"] = []byte(strings.Replace(grading, "%s", path.Base(directory), 1))
			}),
			Entry("grading-channel access only in the follow-on oracle's solution", "gaming.grading_channel_access", func(generated *evalgen.Program) {
				directory := path.Dir(anyKey(generated.FollowOnSolution))
				generated.FollowOnSolution[directory+"/channel.go"] = []byte(strings.Replace(grading, "%s", path.Base(directory), 1))
			}),
			Entry("a forbidden import in the first oracle's solution", "architecture.layer_violation", func(generated *evalgen.Program) {
				name, content := leak(*generated)
				generated.Solution[name] = content
			}),
			Entry("a forbidden import only in the follow-on oracle's solution", "architecture.layer_violation", func(generated *evalgen.Program) {
				name, content := leak(*generated)
				generated.FollowOnSolution[name] = content
			}),
		)

		It("does not read test files, which the static check leaves out (AC-28)", func() {
			generated := cloned(program(0))
			existing := anyKey(generated.Tests)
			packageClause := regexp.MustCompile(`(?m)^package \w+`).FindString(string(generated.Tests[existing]))
			generated.Tests[path.Dir(existing)+"/channel_test.go"] = []byte(packageClause + "\n\nimport \"os\"\n\nvar _ = os.NewFile(3, \"grading\")\n")

			_, err := evalharness.GradeOracles(generated)

			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("native G1", func() {
		DescribeTable("rejects at the g1_native step, naming the test, when a hidden test is skipped (AC-29)",
			func(mutate func(generated *evalgen.Program) string) {
				generated := cloned(program(0))
				skipped := mutate(&generated)

				_, err := evalharness.GradeOracles(generated)

				Expect(err).To(HaveOccurred())
				Expect(rejectionStep(err)).To(Equal(evalgen.StepG1Native))
				Expect(err.Error()).To(ContainSubstring(skipped))
				Expect(err.Error()).To(ContainSubstring("skip"))
			},
			Entry("in the feature suite", func(generated *evalgen.Program) (id string) {
				generated.Tests, id = withSkippedTest(generated.Tests, anyKey(generated.Tests))
				return id
			}),
			Entry("in the follow-on suite", func(generated *evalgen.Program) (id string) {
				generated.FollowOnTests, id = withSkippedTest(generated.FollowOnTests, anyKey(generated.FollowOnTests))
				return id
			}),
		)

		DescribeTable("rejects at the g1_native step, naming go test, when go test fails although every required test passes (AC-29)",
			func(file, source string) {
				generated := cloned(program(0))
				generated.Solution["internal/zz/"+file] = []byte(source)

				_, err := evalharness.GradeOracles(generated)

				Expect(err).To(HaveOccurred())
				Expect(rejectionStep(err)).To(Equal(evalgen.StepG1Native))
				Expect(err.Error()).To(ContainSubstring("go test"))
			},
			Entry("a test no list names fails", "zz_test.go", "package zz\n\nimport \"testing\"\n\nfunc TestUnlisted(t *testing.T) { t.Fatal(\"no\") }\n"),
			Entry("a test file in a package of its own does not build", "zz_test.go", "package zz\n\nfunc TestBroken() { undefinedSymbol() }\n"),
		)

		DescribeTable("counts each oracle's own passing tests, so a hidden test one suite has and the other lacks shows in that count only (AC-28)",
			func(addTo func(generated *evalgen.Program), first, followOn int) {
				generated := cloned(program(0))
				baseline, err := evalharness.GradeOracles(generated)
				Expect(err).NotTo(HaveOccurred())
				addTo(&generated)

				grades, err := evalharness.GradeOracles(generated)

				Expect(err).NotTo(HaveOccurred())
				Expect(grades.FirstOracleTests).To(Equal(baseline.FirstOracleTests + first))
				Expect(grades.FollowOnOracleTests).To(Equal(baseline.FollowOnOracleTests + followOn))
			},
			Entry("one more hidden feature test", func(g *evalgen.Program) { g.Tests = withHiddenTest(g.Tests) }, 1, 0),
			Entry("one more hidden follow-on test", func(g *evalgen.Program) { g.FollowOnTests = withHiddenTest(g.FollowOnTests) }, 0, 1),
		)

		It("rejects when a visible test the manifest lists never reports (AC-29)", func() {
			generated := cloned(program(0))
			visible := &generated.Manifest.Template.VisibleTests[0]
			visible.TestIDs = append(append([]string(nil), visible.TestIDs...), "TestThatDoesNotExist")

			_, err := evalharness.GradeOracles(generated)

			Expect(err).To(HaveOccurred())
			Expect(rejectionStep(err)).To(Equal(evalgen.StepG1Native))
			Expect(err.Error()).To(ContainSubstring("TestThatDoesNotExist"))
			Expect(err.Error()).To(ContainSubstring("did not report"))
		})

		It("rejects a program whose visible test is skipped, which the native step accepts (AC-29)", func() {
			generated := cloned(program(0))
			var name string
			for file := range generated.Tree {
				if strings.HasSuffix(file, "_test.go") && !strings.HasPrefix(file, "tests/") {
					name = file
					break
				}
			}
			Expect(name).NotTo(BeEmpty())
			var skipped string
			generated.Tree, skipped = withSkippedTest(generated.Tree, name)

			_, err := evalharness.GradeOracles(generated)

			Expect(err).To(HaveOccurred())
			Expect(rejectionStep(err)).To(Equal(evalgen.StepG1Native))
			Expect(err.Error()).To(ContainSubstring(skipped))
		})
	})
})
