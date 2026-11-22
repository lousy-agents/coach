package projectconfig

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("the policy the working tree selects", func() {
	const (
		canonicalBytes = `{"schema_version":"1","roots":["."]}`
		legacyBytes    = `{"schema_version":"1","roots":["pkg"]}`
		namedPath      = "policies/team.json"
	)

	var repo string

	put := func(name, contents string) {
		target := filepath.Join(repo, name)
		Expect(os.MkdirAll(filepath.Dir(target), 0o755)).To(Succeed())
		Expect(os.WriteFile(target, []byte(contents), 0o644)).To(Succeed())
	}
	digestOf := func(contents string) string { return Digest([]byte(contents)) }

	BeforeEach(func() {
		repo = gitfixture.Init(GinkgoT())
		gitfixture.CommitFile(GinkgoT(), repo, "README.md", "x")
	})

	When("Coach discovered the policy", func() {
		discovered := ""

		It("selects the canonical file and its digest when both spellings are on disk", func() {
			put(CanonicalPath, canonicalBytes)
			put(DefaultPath, legacyBytes)

			selected, err := SelectOnDisk(repo, discovered)

			Expect(err).NotTo(HaveOccurred())
			Expect(selected).To(Equal(DiskSelection{Path: CanonicalPath, Digest: digestOf(canonicalBytes)}))
		})

		It("selects project.json when the canonical file is absent", func() {
			put(DefaultPath, legacyBytes)

			selected, err := SelectOnDisk(repo, discovered)

			Expect(err).NotTo(HaveOccurred())
			Expect(selected).To(Equal(DiskSelection{Path: DefaultPath, Digest: digestOf(legacyBytes)}))
		})

		It("selects project.json when the canonical path is a symbolic link", func() {
			put(DefaultPath, legacyBytes)
			put("elsewhere.json", canonicalBytes)
			Expect(os.MkdirAll(filepath.Join(repo, ".lousy-agents"), 0o755)).To(Succeed())
			Expect(os.Symlink("../elsewhere.json", filepath.Join(repo, CanonicalPath))).To(Succeed())

			selected, err := SelectOnDisk(repo, discovered)

			Expect(err).NotTo(HaveOccurred())
			Expect(selected).To(Equal(DiskSelection{Path: DefaultPath, Digest: digestOf(legacyBytes)}))
		})

		It("selects nothing when neither spelling is on disk", func() {
			selected, err := SelectOnDisk(repo, discovered)

			Expect(err).NotTo(HaveOccurred())
			Expect(selected).To(Equal(DiskSelection{}))
		})

		It("selects nothing when project.json is a directory", func() {
			Expect(os.MkdirAll(filepath.Join(repo, DefaultPath), 0o755)).To(Succeed())

			selected, err := SelectOnDisk(repo, discovered)

			Expect(err).NotTo(HaveOccurred())
			Expect(selected).To(Equal(DiskSelection{}))
		})

		It("gives the same digest for the same bytes and another for different bytes", func() {
			put(CanonicalPath, canonicalBytes)
			first, err := SelectOnDisk(repo, discovered)
			Expect(err).NotTo(HaveOccurred())
			put(CanonicalPath, canonicalBytes)
			again, err := SelectOnDisk(repo, discovered)
			Expect(err).NotTo(HaveOccurred())
			put(CanonicalPath, legacyBytes)
			edited, err := SelectOnDisk(repo, discovered)
			Expect(err).NotTo(HaveOccurred())

			Expect(again).To(Equal(first))
			Expect(edited.Path).To(Equal(CanonicalPath))
			Expect(edited.Digest).NotTo(Equal(first.Digest))
		})

		It("resolves the repository root from a subdirectory", func() {
			put(CanonicalPath, canonicalBytes)
			Expect(os.MkdirAll(filepath.Join(repo, "pkg", "deep"), 0o755)).To(Succeed())

			selected, err := SelectOnDisk(filepath.Join(repo, "pkg", "deep"), discovered)

			Expect(err).NotTo(HaveOccurred())
			Expect(selected).To(Equal(DiskSelection{Path: CanonicalPath, Digest: digestOf(canonicalBytes)}))
		})

		It("selects a canonical file of exactly the size bound", func() {
			atBound := strings.Repeat(" ", MaxBytes)
			put(CanonicalPath, atBound)

			selected, err := SelectOnDisk(repo, discovered)

			Expect(err).NotTo(HaveOccurred())
			Expect(selected).To(Equal(DiskSelection{Path: CanonicalPath, Digest: digestOf(atBound)}))
		})

		It("refuses a canonical file over the size bound and selects nothing", func() {
			put(CanonicalPath, strings.Repeat("a", MaxBytes+1))

			selected, err := SelectOnDisk(repo, discovered)

			Expect(err).To(HaveOccurred())
			Expect(selected).To(Equal(DiskSelection{}))
		})

		It("refuses a canonical file the process cannot read and selects nothing", func() {
			if os.Geteuid() == 0 {
				Skip("root can read any file")
			}
			put(CanonicalPath, canonicalBytes)
			Expect(os.Chmod(filepath.Join(repo, CanonicalPath), 0o000)).To(Succeed())

			selected, err := SelectOnDisk(repo, discovered)

			Expect(err).To(HaveOccurred())
			Expect(selected).To(Equal(DiskSelection{}))
		})

		It("selects project.json when a regular file stands where the canonical parent directory should be", func() {
			put(".lousy-agents", "x")
			put(DefaultPath, legacyBytes)

			selected, err := SelectOnDisk(repo, discovered)

			Expect(err).NotTo(HaveOccurred())
			Expect(selected).To(Equal(DiskSelection{Path: DefaultPath, Digest: digestOf(legacyBytes)}))
		})
	})

	When("the user named the policy", func() {
		named := namedPath

		It("selects the named regular file even when the canonical file is also on disk", func() {
			put(namedPath, legacyBytes)
			put(CanonicalPath, canonicalBytes)

			selected, err := SelectOnDisk(repo, named)

			Expect(err).NotTo(HaveOccurred())
			Expect(selected).To(Equal(DiskSelection{Path: namedPath, Digest: digestOf(legacyBytes)}))
		})

		It("selects nothing when the named file is missing, though project.json is on disk", func() {
			put(DefaultPath, legacyBytes)

			selected, err := SelectOnDisk(repo, named)

			Expect(err).NotTo(HaveOccurred())
			Expect(selected).To(Equal(DiskSelection{}))
		})

		It("selects nothing when the named file is a symbolic link", func() {
			put("real.json", legacyBytes)
			Expect(os.MkdirAll(filepath.Join(repo, "policies"), 0o755)).To(Succeed())
			Expect(os.Symlink("../real.json", filepath.Join(repo, namedPath))).To(Succeed())

			selected, err := SelectOnDisk(repo, named)

			Expect(err).NotTo(HaveOccurred())
			Expect(selected).To(Equal(DiskSelection{}))
		})

		It("selects nothing when the named file is reached through a symbolic link to a directory", func() {
			put("real/team.json", legacyBytes)
			Expect(os.Symlink("real", filepath.Join(repo, "policies"))).To(Succeed())

			selected, err := SelectOnDisk(repo, named)

			Expect(err).NotTo(HaveOccurred())
			Expect(selected).To(Equal(DiskSelection{}))
		})

		It("selects nothing when a regular file stands where the named file's parent directory should be", func() {
			put("policies", "x")

			selected, err := SelectOnDisk(repo, named)

			Expect(err).NotTo(HaveOccurred())
			Expect(selected).To(Equal(DiskSelection{}))
		})

		It("refuses a named path that escapes the repository", func() {
			outside := filepath.Join(filepath.Dir(repo), filepath.Base(repo)+"-outside.json")
			Expect(os.WriteFile(outside, []byte(legacyBytes), 0o644)).To(Succeed())
			DeferCleanup(func() { _ = os.Remove(outside) })
			put(DefaultPath, legacyBytes)

			selected, err := SelectOnDisk(repo, "../"+filepath.Base(outside))

			Expect(err).To(HaveOccurred())
			Expect(selected).To(Equal(DiskSelection{}))
		})

		It("selects the named file when asked from a subdirectory", func() {
			put(namedPath, legacyBytes)
			Expect(os.MkdirAll(filepath.Join(repo, "pkg"), 0o755)).To(Succeed())

			selected, err := SelectOnDisk(filepath.Join(repo, "pkg"), named)

			Expect(err).NotTo(HaveOccurred())
			Expect(selected).To(Equal(DiskSelection{Path: namedPath, Digest: digestOf(legacyBytes)}))
		})

		It("selects a named file of exactly the size bound", func() {
			atBound := strings.Repeat(" ", MaxBytes)
			put(namedPath, atBound)

			selected, err := SelectOnDisk(repo, named)

			Expect(err).NotTo(HaveOccurred())
			Expect(selected).To(Equal(DiskSelection{Path: namedPath, Digest: digestOf(atBound)}))
		})

		It("refuses a named file over the size bound", func() {
			put(namedPath, strings.Repeat("a", MaxBytes+1))

			selected, err := SelectOnDisk(repo, named)

			Expect(err).To(HaveOccurred())
			Expect(selected).To(Equal(DiskSelection{}))
		})
	})
})
