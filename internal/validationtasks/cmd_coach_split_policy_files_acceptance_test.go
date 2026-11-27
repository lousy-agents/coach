package validationtasks

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func specFilesMatching(dir, pattern string) []string {
	GinkgoHelper()
	focus := regexp.MustCompile(pattern)
	files, err := filepath.Glob(filepath.Join(dir, "*_test.go"))
	Expect(err).NotTo(HaveOccurred())
	var matched []string
	for _, file := range files {
		if focus.MatchString(filepath.ToSlash(file)) && holdsASpec(file) {
			matched = append(matched, filepath.Base(file))
		}
	}
	return matched
}

func holdsASpec(file string) bool {
	GinkgoHelper()
	raw, err := os.ReadFile(file)
	Expect(err).NotTo(HaveOccurred())
	return strings.Contains(string(raw), "It(") || strings.Contains(string(raw), "Entry(")
}

var _ = Describe("the policy half's selection of cmd/coach's spec files (#920)", func() {
	var toml string

	BeforeEach(func() {
		toml = readRepoText("mise.toml")
	})

	It("selects at least one spec file of cmd/coach with the pattern, so the policy leg cannot go green with no specs when the files are renamed", func() {
		pattern := flagValues(policyCommand(toml), "ginkgo.focus-file")
		Expect(pattern).To(HaveLen(1))

		Expect(specFilesMatching(filepath.Join("..", "..", "cmd", "coach"), pattern[0])).NotTo(BeEmpty())
	})

	It("sees a pattern that no spec file matches", func() {
		Expect(specFilesMatching(filepath.Join("..", "..", "cmd", "coach"), "no_such_file_name_")).To(BeEmpty())
	})

	It("does not count a file the pattern matches when it holds no spec", func() {
		dir := GinkgoT().TempDir()
		Expect(os.WriteFile(filepath.Join(dir, "policy_helpers_test.go"), []byte("package x\n\nfunc helper() {}\n"), 0o644)).To(Succeed())

		Expect(specFilesMatching(dir, "policy_")).To(BeEmpty())
	})

	It("counts a file the pattern matches that holds a spec as an It or as a table Entry, and no file the pattern does not match", func() {
		dir := GinkgoT().TempDir()
		Expect(os.WriteFile(filepath.Join(dir, "policy_entry_test.go"), []byte("package x\n\nvar _ = DescribeTable(\"t\", f, Entry(\"e\", 1))\n"), 0o644)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(dir, "policy_it_test.go"), []byte("package x\n\nvar _ = Describe(\"d\", func() { It(\"i\", f) })\n"), 0o644)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(dir, "other_it_test.go"), []byte("package x\n\nvar _ = Describe(\"d\", func() { It(\"i\", f) })\n"), 0o644)).To(Succeed())

		Expect(specFilesMatching(dir, "policy_")).To(Equal([]string{"policy_entry_test.go", "policy_it_test.go"}))
	})

	It("fails the policy half when it selects no spec, with -ginkgo.fail-on-empty", func() {
		Expect(strings.Fields(policyCommand(toml))).To(ContainElement("-ginkgo.fail-on-empty"))
	})
})
