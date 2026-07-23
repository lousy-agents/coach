package acceptancestyle_test

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/acceptancestyle"
)

const ginkgoImport = `"github.com/onsi/ginkgo/v2"`

func writeFile(dir, rel, body string) string {
	GinkgoHelper()
	path := filepath.Join(dir, filepath.FromSlash(rel))
	Expect(os.MkdirAll(filepath.Dir(path), 0o755)).To(Succeed())
	Expect(os.WriteFile(path, []byte(body), 0o644)).To(Succeed())
	return path
}

var _ = Describe("acceptance-style checker", func() {
	var root string

	BeforeEach(func() {
		root = GinkgoT().TempDir()
	})

	When("a tree contains foo_acceptance_test.go that does not import ginkgo/v2", func() {
		It("reports a violation naming that path", func() {
			writeFile(root, "pkg/foo/foo_acceptance_test.go", `package foo_test

import "testing"

func TestFooAcceptance(t *testing.T) {}
`)

			violations, err := acceptancestyle.Check(root)

			Expect(err).NotTo(HaveOccurred())
			Expect(violations).NotTo(BeEmpty())
			joined := violationsPaths(violations)
			Expect(joined).To(ContainSubstring("foo_acceptance_test.go"))
			Expect(violations[0].Reason).NotTo(BeEmpty())
		})
	})

	When("a tree contains foo_acceptance_test.go that imports ginkgo/v2 and calls RunSpecs", func() {
		It("reports no violations", func() {
			writeFile(root, "pkg/foo/foo_acceptance_test.go", `package foo_test

import (
	"testing"

	. `+ginkgoImport+`
)

func TestFooAcceptance(t *testing.T) {
	RunSpecs(t, "foo")
}
`)

			violations, err := acceptancestyle.Check(root)

			Expect(err).NotTo(HaveOccurred())
			Expect(violations).To(BeEmpty())
		})
	})

	When("a tree contains foo_acceptance_test.go with only a blank ginkgo import", func() {
		It("reports a violation", func() {
			writeFile(root, "pkg/foo/foo_acceptance_test.go", `package foo_test

import (
	"testing"

	_ `+ginkgoImport+`
)

func TestFooAcceptance(t *testing.T) {}
`)

			violations, err := acceptancestyle.Check(root)

			Expect(err).NotTo(HaveOccurred())
			Expect(violations).NotTo(BeEmpty())
			Expect(violationsPaths(violations)).To(ContainSubstring("foo_acceptance_test.go"))
		})
	})

	When("a tree contains foo_acceptance_test.go that imports ginkgo but has no specs", func() {
		It("reports a violation", func() {
			writeFile(root, "pkg/foo/foo_acceptance_test.go", `package foo_test

import (
	"testing"

	. `+ginkgoImport+`
)

func TestFooAcceptance(t *testing.T) {}
`)

			violations, err := acceptancestyle.Check(root)

			Expect(err).NotTo(HaveOccurred())
			Expect(violations).NotTo(BeEmpty())
			Expect(violations[0].Reason).To(ContainSubstring("suite/spec API"))
		})
	})

	When("a tree contains only a non-acceptance *_test.go without ginkgo", func() {
		It("ignores that file", func() {
			writeFile(root, "pkg/foo/foo_test.go", `package foo_test

import "testing"

func TestFoo(t *testing.T) {}
`)

			violations, err := acceptancestyle.Check(root)

			Expect(err).NotTo(HaveOccurred())
			Expect(violations).To(BeEmpty())
		})
	})

	When("a tree contains a non-allowlisted acceptance_test.go without ginkgo", func() {
		It("reports a violation naming that path", func() {
			// Bare basename acceptance_test.go must be scanned (not only *_acceptance_test.go).
			writeFile(root, "pkg/other/acceptance_test.go", `package other_test

import "testing"

func TestOtherAcceptance(t *testing.T) {}
`)

			violations, err := acceptancestyle.Check(root)

			Expect(err).NotTo(HaveOccurred())
			Expect(violations).NotTo(BeEmpty())
			Expect(violationsPaths(violations)).To(ContainSubstring("acceptance_test.go"))
			Expect(violations[0].Reason).NotTo(BeEmpty())
		})
	})

	When("a tree contains an allowlisted acceptance_test.go without ginkgo", func() {
		It("reports no violations for that path because allowlist exempts it", func() {
			// Same relative path as the v1 allowlist entry for the thin
			// stdlib Test*Acceptance wrapper around queueconformance.Run.
			// Content lacks ginkgo: empty result only if allowlist is consulted.
			rel := "internal/acceptanceharness/queueconformance/acceptance_test.go"
			writeFile(root, rel, `package queueconformance

import "testing"

func TestQueueConformanceAcceptance(t *testing.T) {}
`)

			violations, err := acceptancestyle.Check(root)

			Expect(err).NotTo(HaveOccurred())
			Expect(violations).To(BeEmpty())
		})
	})

	When("a tree contains an allowlisted compose_acceptance_test.go without ginkgo", func() {
		It("reports no violations for that path because allowlist exempts it", func() {
			rel := "internal/acceptanceharness/thinproof/compose_acceptance_test.go"
			writeFile(root, rel, `package thinproof

import "testing"

func TestComposeAcceptance(t *testing.T) {}
`)

			violations, err := acceptancestyle.Check(root)

			Expect(err).NotTo(HaveOccurred())
			Expect(violations).To(BeEmpty())
		})
	})
})

func violationsPaths(vs []acceptancestyle.Violation) string {
	var b strings.Builder
	for _, v := range vs {
		b.WriteString(v.Path)
		b.WriteByte('\n')
	}
	return b.String()
}
