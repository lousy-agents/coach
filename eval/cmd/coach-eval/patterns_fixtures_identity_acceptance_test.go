package main_test

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// committedProfiles copies eval/patterns/profiles into a fresh directory,
// with the coach-self profile changed by change when one is given.
func committedProfiles(change func(profile map[string]any)) string {
	GinkgoHelper()
	raw := committedProfile()
	if change != nil {
		raw = edited(raw, false, change)
	}
	profiles := filepath.Join(GinkgoT().TempDir(), "profiles")
	Expect(os.MkdirAll(profiles, 0o755)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(profiles, "coach-self.profile.json"), raw, 0o644)).To(Succeed())
	return profiles
}

// treeOf is every file under root, keyed by its path relative to root.
func treeOf(root string) map[string]string {
	GinkgoHelper()
	tree := map[string]string{}
	Expect(filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		raw, err := os.ReadFile(path)
		rel, _ := filepath.Rel(root, path)
		tree[rel] = string(raw)
		return err
	})).To(Succeed())
	return tree
}

var _ = Describe("coach-eval patterns fixtures checks each profile's identity (#912)", func() {
	var out string
	BeforeEach(func() { out = filepath.Join(GinkgoT().TempDir(), "derived") })

	It("accepts the committed coach-self profile and derives the committed fixtures, byte for byte", func() {
		outcome := runPatternsFixtures("--profiles", committedProfiles(nil), "--out", out)

		Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
		committed := treeOf(filepath.Join(repositoryRoot(), "eval", "fixtures", "antipatterns-derived", "coach-self"))
		Expect(committed).NotTo(BeEmpty())
		Expect(treeOf(filepath.Join(out, "coach-self"))).To(Equal(committed))
	})

	When("a profile's counts were edited and its identity was not", func() {
		It("refuses with exit 2, naming the file and both identities, before anything under --out is removed", func() {
			profiles := committedProfiles(oneMoreLayer)
			stale, err := os.ReadFile(filepath.Join(profiles, "coach-self.profile.json"))
			Expect(err).NotTo(HaveOccurred())
			var decoded map[string]any
			Expect(json.Unmarshal(stale, &decoded)).To(Succeed())
			kept := filepath.Join(out, "coach-self", "kept.txt")
			Expect(os.MkdirAll(filepath.Dir(kept), 0o755)).To(Succeed())
			Expect(os.WriteFile(kept, []byte("kept\n"), 0o644)).To(Succeed())

			outcome := runPatternsFixtures("--profiles", profiles, "--out", out)

			Expect(outcome.exitCode).To(Equal(2), outcome.stdout)
			Expect(outcome.stderr).To(ContainSubstring("coach-self.profile.json"))
			Expect(outcome.stderr).To(ContainSubstring(profileIdentity(committedProfile())))
			Expect(outcome.stderr).To(ContainSubstring(contentIdentity(decoded)))
			Expect(kept).To(BeAnExistingFile(), "the label's own directory is not cleared")
			Expect(ids(filepath.Join(out, "coach-self"))).To(Equal([]string{"kept.txt"}))
		})
	})
})
