package projectconfig

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("the policy the working tree selects for a discovered scan (#168 A2b)", func() {
	const (
		canonicalBytes = `{"schema_version":"1","roots":["."]}`
		legacyBytes    = `{"schema_version":"1","roots":["pkg"]}`
	)

	var repo string
	put := func(name, contents string) {
		target := filepath.Join(repo, name)
		Expect(os.MkdirAll(filepath.Dir(target), 0o755)).To(Succeed())
		Expect(os.WriteFile(target, []byte(contents), 0o644)).To(Succeed())
	}

	BeforeEach(func() {
		repo = gitfixture.Init(GinkgoT())
		gitfixture.CommitFile(GinkgoT(), repo, "README.md", "x")
	})

	DescribeTable("DiskSelection.Deprecated is true exactly for the root project.json",
		func(path string, want bool) {
			Expect(DiskSelection{Path: path}.Deprecated()).To(Equal(want))
		},
		Entry("the root project.json", DefaultPath, true),
		Entry("the canonical path", CanonicalPath, false),
		Entry("a named path", "policies/team.json", false),
		Entry("no policy", "", false),
	)

	It("selects the canonical file, which is not deprecated, when both spellings are on disk", func() {
		put(CanonicalPath, canonicalBytes)
		put(DefaultPath, legacyBytes)

		selected, err := DiscoverOnDisk(repo, Selection{})

		Expect(err).NotTo(HaveOccurred())
		Expect(selected.Path).To(Equal(CanonicalPath))
		Expect(selected.Deprecated()).To(BeFalse())
	})

	It("selects project.json, which is deprecated, when it is the only spelling on disk", func() {
		put(DefaultPath, legacyBytes)

		selected, err := DiscoverOnDisk(repo, Selection{})

		Expect(err).NotTo(HaveOccurred())
		Expect(selected).To(Equal(DiskSelection{Path: DefaultPath, Digest: Digest([]byte(legacyBytes)), Content: []byte(legacyBytes)}))
		Expect(selected.Deprecated()).To(BeTrue())
	})

	It("selects nothing and reports no error when neither spelling is on disk", func() {
		selected, err := DiscoverOnDisk(repo, Selection{})

		Expect(err).NotTo(HaveOccurred())
		Expect(selected).To(Equal(DiskSelection{}))
	})

	It("refuses a canonical directory as a not_a_regular_file DiskPolicyRefusal and does not fall through to project.json", func() {
		put(DefaultPath, legacyBytes)
		Expect(os.MkdirAll(filepath.Join(repo, CanonicalPath), 0o755)).To(Succeed())

		selected, err := DiscoverOnDisk(repo, Selection{})

		var refusal *DiskPolicyRefusal
		Expect(errors.As(err, &refusal)).To(BeTrue(), "error: %v", err)
		Expect(refusal.Path).To(Equal(CanonicalPath))
		Expect(refusal.Cause()).To(Equal(projectreadiness.CauseNotARegularFile))
		Expect(selected).To(Equal(DiskSelection{}))
	})

	It("refuses a canonical symbolic link as a symbolic_link DiskPolicyRefusal and does not fall through to project.json", func() {
		put(DefaultPath, legacyBytes)
		put("elsewhere.json", canonicalBytes)
		Expect(os.MkdirAll(filepath.Join(repo, ".lousy-agents"), 0o755)).To(Succeed())
		Expect(os.Symlink("../elsewhere.json", filepath.Join(repo, CanonicalPath))).To(Succeed())

		selected, err := DiscoverOnDisk(repo, Selection{})

		var refusal *DiskPolicyRefusal
		Expect(errors.As(err, &refusal)).To(BeTrue(), "error: %v", err)
		Expect(refusal.Cause()).To(Equal(projectreadiness.CauseSymbolicLink))
		Expect(selected).To(Equal(DiskSelection{}))
	})

	It("refuses with the cause in_submodule when the canonical path is committed inside a submodule, though a file is on disk", func() {
		put(CanonicalPath, canonicalBytes)
		committed := Selection{UnusableCanonical: &ConfigError{PolicyCause: projectreadiness.CauseInSubmodule, Blocker: "vendor/sub"}}

		selected, err := DiscoverOnDisk(repo, committed)

		var refusal *ConfigError
		Expect(errors.As(err, &refusal)).To(BeTrue(), "error: %v", err)
		Expect(refusal.PolicyCause).To(Equal(projectreadiness.CauseInSubmodule))
		Expect(selected).To(Equal(DiskSelection{}))
	})

	It("does not let a committed canonical path that is unusable for another reason hide a regular file on disk", func() {
		put(CanonicalPath, canonicalBytes)
		committed := Selection{UnusableCanonical: &ConfigError{PolicyCause: projectreadiness.CauseSymbolicLink}}

		selected, err := DiscoverOnDisk(repo, committed)

		Expect(err).NotTo(HaveOccurred())
		Expect(selected.Path).To(Equal(CanonicalPath))
	})
})
