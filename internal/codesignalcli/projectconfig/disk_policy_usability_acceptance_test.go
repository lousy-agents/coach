package projectconfig

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("the policy on disk, read once and judged from the bytes read", func() {
	const (
		validBytes   = `{"schema_version":"1","roots":["."]}`
		legacyBytes  = `{"schema_version":"1","roots":["pkg"]}`
		malformed    = `{"schema_version":`
		wrongVersion = `{"schema_version":"2","roots":["."]}`
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

	When("a policy file is selected", func() {
		It("carries the exact bytes it read", func() {
			put(CanonicalPath, legacyBytes)

			selected, err := SelectOnDisk(repo, "")

			Expect(err).NotTo(HaveOccurred())
			Expect(string(selected.Content)).To(Equal(legacyBytes))
		})

		It("carries a file of exactly the size bound whole", func() {
			atBound := strings.Repeat(" ", MaxBytes)
			put(CanonicalPath, atBound)

			selected, err := SelectOnDisk(repo, "")

			Expect(err).NotTo(HaveOccurred())
			Expect(selected.Content).To(HaveLen(MaxBytes))
		})

		It("digests the content it carries, not another read of the path", func() {
			put(DefaultPath, validBytes)

			selected, err := SelectOnDisk(repo, "")

			Expect(err).NotTo(HaveOccurred())
			Expect(selected.Digest).To(Equal(Digest(selected.Content)))
			put(DefaultPath, legacyBytes)
			Expect(selected.Digest).To(Equal(Digest([]byte(validBytes))))
		})

		It("carries no content when it selects nothing or refuses", func() {
			none, err := SelectOnDisk(repo, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(none.Content).To(BeEmpty())

			put(CanonicalPath, strings.Repeat("a", MaxBytes+1))
			refused, err := SelectOnDisk(repo, "")
			Expect(err).To(HaveOccurred())
			Expect(refused).To(Equal(DiskSelection{}))
		})
	})

	When("the selected content is judged", func() {
		It("returns the parsed policy for valid content", func() {
			config, err := UsableDiskPolicy(DiskSelection{Path: CanonicalPath, Content: []byte(legacyBytes)})

			Expect(err).NotTo(HaveOccurred())
			Expect(config.Roots).To(Equal([]string{"pkg"}))
		})

		It("judges the bytes it was handed and not the file at the path", func() {
			put(CanonicalPath, validBytes)

			config, err := UsableDiskPolicy(DiskSelection{Path: CanonicalPath, Content: []byte(legacyBytes)})

			Expect(err).NotTo(HaveOccurred())
			Expect(config.Roots).To(Equal([]string{"pkg"}))
		})

		DescribeTable("refuses content that is not a valid project policy",
			func(content []byte) {
				config, err := UsableDiskPolicy(DiskSelection{Path: CanonicalPath, Content: content})

				Expect(err).To(HaveOccurred())
				Expect(config).To(Equal(Config{}))
				var refusal *DiskPolicyRefusal
				Expect(errors.As(ClassifyDiskPolicyError(CanonicalPath, err), &refusal)).To(BeTrue())
				Expect(refusal.Reason).To(Equal("not a valid project policy"))
			},
			Entry("malformed JSON", []byte(malformed)),
			Entry("well-formed JSON the schema rejects", []byte(wrongVersion)),
			Entry("duplicate keys", []byte(`{"schema_version":"1","schema_version":"1","roots":["."]}`)),
			Entry("nesting beyond the depth limit", []byte(`{"schema_version":"1","roots":["."],"x":`+strings.Repeat("[", 200)+strings.Repeat("]", 200)+`}`)),
			Entry("content over the size limit", []byte(strings.Repeat(" ", MaxBytes+1)+validBytes)),
			Entry("no content at all", nil),
		)
	})

	When("a refusal is classified", func() {
		It("calls an oversize file larger than the policy size limit, with the path it was given", func() {
			put(CanonicalPath, strings.Repeat("a", MaxBytes+1))
			_, err := SelectOnDisk(repo, "")

			refusal := ClassifyDiskPolicyError(CanonicalPath, err)

			Expect(refusal).NotTo(BeNil())
			Expect(*refusal).To(Equal(DiskPolicyRefusal{Path: CanonicalPath, Reason: "larger than the policy size limit"}))
		})

		It("calls a named path that escapes the repository unreadable", func() {
			_, err := SelectOnDisk(repo, "../outside.json")
			Expect(err).To(HaveOccurred())

			refusal := ClassifyDiskPolicyError("../outside.json", err)

			Expect(refusal).NotTo(BeNil())
			Expect(*refusal).To(Equal(DiskPolicyRefusal{Path: "../outside.json", Reason: "unreadable"}))
		})

		It("calls any other failure unreadable", func() {
			refusal := ClassifyDiskPolicyError(DefaultPath, errors.New("input/output error"))

			Expect(*refusal).To(Equal(DiskPolicyRefusal{Path: DefaultPath, Reason: "unreadable"}))
		})

		It("is nothing for no error", func() {
			Expect(ClassifyDiskPolicyError(DefaultPath, nil)).To(BeNil())
		})

		It("words itself as the reason after the path so a caller can print it", func() {
			refusal := &DiskPolicyRefusal{Path: DefaultPath, Reason: "unreadable"}

			Expect(refusal.Error()).To(Equal("project.json: unreadable"))
		})
	})
})
