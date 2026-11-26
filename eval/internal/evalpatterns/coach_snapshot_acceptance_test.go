package evalpatterns_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalpatterns"
)

// coachSnapshotSHA is the commit of Coach's own repository that stands in
// for a reference repository until the owner shares one (#872).
const coachSnapshotSHA = "d21678d6a41b9b7d81e7b9e1ce568d086a644c20"

func repositoryRoot() string {
	GinkgoHelper()
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	Expect(err).NotTo(HaveOccurred())
	return strings.TrimSpace(string(out))
}

var _ = Describe("Coach's own repository as a stand-in reference (AC-1.1, AC-1.3, AC-1.4)", func() {
	It("checks the pinned commit out in isolation and extracts the reviewed snapshot, byte for byte", func() {
		reference := evalpatterns.Reference{Label: "coach-self", SHA: coachSnapshotSHA, Language: "go", Licence: "BSD-2-Clause"}
		dir := filepath.Join(GinkgoT().TempDir(), "checkout")
		Expect(evalpatterns.CheckoutReference(reference, repositoryRoot(), dir)).To(Succeed())

		raw, err := evalpatterns.ExtractProfile(reference, dir)
		Expect(err).NotTo(HaveOccurred())

		snapshot, err := os.ReadFile(filepath.Join("testdata", "coach-self.profile.json"))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(raw)).To(Equal(string(snapshot)))
		Expect(evalpatterns.ValidateProfile(raw)).To(Succeed())
		for _, name := range []string{"codesignal", "semantics", "githubingest", "evalpatterns", "lousy-agents", "pkg/", "cmd/", ".go"} {
			Expect(string(raw)).NotTo(ContainSubstring(name))
		}
	})
})
