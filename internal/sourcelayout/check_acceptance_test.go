package sourcelayout_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/sourcelayout"
)

const cleanGoFile = "package foo\n\nfunc parseFlags() {}\n"

func writeGoFile(root, rel, body string) {
	GinkgoHelper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	Expect(os.MkdirAll(filepath.Dir(path), 0o755)).To(Succeed())
	Expect(os.WriteFile(path, []byte(body), 0o644)).To(Succeed())
}

func violationPaths(violations []sourcelayout.Violation) []string {
	paths := make([]string, 0, len(violations))
	for _, v := range violations {
		paths = append(paths, v.Path)
	}
	return paths
}

var _ = Describe("source-layout checker", func() {
	var root string

	BeforeEach(func() {
		root = GinkgoT().TempDir()
	})

	DescribeTable("rejects a Go file named by position instead of responsibility",
		func(name string) {
			writeGoFile(root, "pkg/foo/"+name, cleanGoFile)

			violations, err := sourcelayout.Check(root)

			Expect(err).NotTo(HaveOccurred())
			Expect(violationPaths(violations)).To(ConsistOf("pkg/foo/" + name))
		},
		Entry("an ordinal production fragment", "main_part2.go"),
		Entry("an ordinal test fragment", "main_part3_test.go"),
		Entry("an extracted test body", "flags_test_body_test.go"),
		Entry("a numbered extracted test body", "flags_test_body2_test.go"),
		Entry("a signature-fix fragment", "proxy_acceptance_sigfix1_test.go"),
	)

	DescribeTable("rejects identifiers named by origin or hash instead of behavior",
		func(body string) {
			writeGoFile(root, "pkg/foo/flags_test.go", body)

			violations, err := sourcelayout.Check(root)

			Expect(err).NotTo(HaveOccurred())
			Expect(violationPaths(violations)).To(ConsistOf("pkg/foo/flags_test.go"))
		},
		Entry("a positional body_ helper", "package foo\n\nfunc body_mainPart2Test_44() {}\n"),
		Entry("a hash-suffixed type", "package foo\n\ntype sigstartRecordingProxyListener39957725 struct{}\n"),
		Entry("a hash-suffixed local", "package foo\n\nfunc f() { v61725567 := 1; _ = v61725567 }\n"),
	)

	When("every Go file and identifier names a responsibility", func() {
		It("reports no violations, including names that merely contain part or body", func() {
			writeGoFile(root, "pkg/foo/flags_validate.go", cleanGoFile)
			writeGoFile(root, "pkg/foo/request_body_test.go", "package foo\n\nfunc requestBody() string { return \"\" }\n")
			writeGoFile(root, "pkg/foo/partition.go", "package foo\n\nvar sha256Sum, crc32 = 1, 2\n")

			violations, err := sourcelayout.Check(root)

			Expect(err).NotTo(HaveOccurred())
			Expect(violations).To(BeEmpty())
		})
	})

	When("a fragment sits under a vendored or testdata directory", func() {
		It("is not this repository's source and is not reported", func() {
			writeGoFile(root, "vendor/x/main_part2.go", cleanGoFile)
			writeGoFile(root, "pkg/foo/testdata/fixture_part2.go", cleanGoFile)

			violations, err := sourcelayout.Check(root)

			Expect(err).NotTo(HaveOccurred())
			Expect(violations).To(BeEmpty())
		})
	})

	When("run against this repository", func() {
		It("finds every Go source file named for its responsibility", func() {
			repoRoot, err := filepath.Abs("../..")
			Expect(err).NotTo(HaveOccurred())

			violations, err := sourcelayout.Check(repoRoot)

			Expect(err).NotTo(HaveOccurred())
			Expect(violations).To(BeEmpty(), "see docs/architecture/ADR-007-layered-ports-and-adapters.md")
		})
	})
})
