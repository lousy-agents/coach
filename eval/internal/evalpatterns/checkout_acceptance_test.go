package evalpatterns_test

import (
	"errors"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalpatterns"
)

// plantMarkerScript writes an executable script that creates marker when it
// runs, and returns the script's path.
func plantMarkerScript(dir, marker string) string {
	GinkgoHelper()
	script := filepath.Join(dir, "plant.sh")
	writeFile(script, "#!/bin/sh\ntouch '"+marker+"'\ncat\n")
	Expect(os.Chmod(script, 0o755)).To(Succeed())
	return script
}

var _ = Describe("checking out a reference", func() {
	When("the entry's SHA is reachable in the local clone", func() {
		It("checks the SHA out into a fresh directory", func() {
			clone, sha := referenceRepository()
			reference := validReference()
			reference.SHA = sha
			dir := filepath.Join(GinkgoT().TempDir(), "checkout")

			Expect(evalpatterns.CheckoutReference(reference, clone, dir)).To(Succeed())

			Expect(filepath.Join(dir, "main.go")).To(BeAnExistingFile())
			Expect(runGit(dir, "rev-parse", "HEAD")).To(Equal(sha))
		})
	})

	When("the entry's SHA is 40-hex but not reachable in the local clone", func() {
		It("refuses the entry with exit 2, naming its label (AC-1.6)", func() {
			clone, _ := referenceRepository()

			err := evalpatterns.CheckoutReference(validReference(), clone, filepath.Join(GinkgoT().TempDir(), "checkout"))

			var refusal evalpatterns.Refusal
			Expect(errors.As(err, &refusal)).To(BeTrue(), "CheckoutReference returned %v", err)
			Expect(refusal.Exit).To(Equal(2))
			Expect(refusal.Label).To(Equal("stand-in"))
			Expect(err.Error()).To(HavePrefix(`reference "stand-in": `))
			Expect(err.Error()).To(ContainSubstring("SHA " + sampleSHA + " is not reachable"))
		})
	})

	DescribeTable("refuses a clone that is not a local directory with exit 2, naming the label, before running git",
		func(clone func(local string) string, reason string) {
			local, sha := referenceRepository()
			reference := validReference()
			reference.SHA = sha
			dir := filepath.Join(GinkgoT().TempDir(), "checkout")

			err := evalpatterns.CheckoutReference(reference, clone(local), dir)

			var refusal evalpatterns.Refusal
			Expect(errors.As(err, &refusal)).To(BeTrue(), "CheckoutReference returned %v", err)
			Expect(refusal.Exit).To(Equal(2))
			Expect(err.Error()).To(HavePrefix(`reference "stand-in": `))
			Expect(err.Error()).To(ContainSubstring(reason))
			Expect(dir).NotTo(BeADirectory())
		},
		Entry("an https URL, which would open a network connection",
			func(string) string { return "https://127.0.0.1:1/x.git" }, `is a URL (scheme "https")`),
		Entry("a file URL to the local clone, which git would read through upload-pack",
			func(local string) string { return "file://" + local }, `is a URL (scheme "file")`),
		Entry("an ssh URL",
			func(string) string { return "ssh://example.invalid/x.git" }, `is a URL (scheme "ssh")`),
		Entry("an scp-style host and path",
			func(string) string { return "example.invalid:x.git" }, "is not a local directory"),
		Entry("a path that does not exist",
			func(local string) string { return filepath.Join(local, "missing") }, "is not a local directory"),
		Entry("a regular file",
			func(local string) string { return filepath.Join(local, "main.go") }, "is not a local directory"),
	)

	When("the clone is given as a relative path to a local directory", func() {
		It("checks the SHA out", func() {
			local, sha := referenceRepository()
			reference := validReference()
			reference.SHA = sha
			dir := filepath.Join(GinkgoT().TempDir(), "checkout")
			GinkgoT().Chdir(filepath.Dir(local))

			Expect(evalpatterns.CheckoutReference(reference, filepath.Base(local), dir)).To(Succeed())

			Expect(filepath.Join(dir, "main.go")).To(BeAnExistingFile())
		})
	})

	When("the clone is a local directory whose name has a colon before any slash", func() {
		It("reads it from the disk, not as an scp-style ssh address", func() {
			local, sha := referenceRepository()
			parent := GinkgoT().TempDir()
			Expect(os.Rename(local, filepath.Join(parent, "host:repo"))).To(Succeed())
			reference := validReference()
			reference.SHA = sha
			dir := filepath.Join(GinkgoT().TempDir(), "checkout")
			GinkgoT().Chdir(parent)

			Expect(evalpatterns.CheckoutReference(reference, "host:repo", dir)).To(Succeed())

			Expect(filepath.Join(dir, "main.go")).To(BeAnExistingFile())
		})
	})

	When("the entry itself is refused", func() {
		It("refuses before cloning anything (AC-1.5)", func() {
			clone, sha := referenceRepository()
			reference := validReference()
			reference.SHA = sha
			reference.Licence = ""
			dir := filepath.Join(GinkgoT().TempDir(), "checkout")

			err := evalpatterns.CheckoutReference(reference, clone, dir)

			Expect(err).To(MatchError(`reference "stand-in": licence is required`))
			Expect(dir).NotTo(BeADirectory())
		})
	})

	When("the clone and the host define a hook and a smudge filter (AC-1.8)", func() {
		It("runs neither, so the checked-out file is the committed bytes", func() {
			clone, _ := referenceRepository()
			planted := GinkgoT().TempDir()
			script := plantMarkerScript(planted, filepath.Join(planted, "ran"))
			hooks := filepath.Join(planted, "hooks")
			writeFile(filepath.Join(hooks, "post-checkout"), "#!/bin/sh\ntouch '"+filepath.Join(planted, "ran")+"'\n")
			Expect(os.Chmod(filepath.Join(hooks, "post-checkout"), 0o755)).To(Succeed())
			writeFile(filepath.Join(clone, ".git", "hooks", "post-checkout"), "#!/bin/sh\ntouch '"+filepath.Join(planted, "ran")+"'\n")
			Expect(os.Chmod(filepath.Join(clone, ".git", "hooks", "post-checkout"), 0o755)).To(Succeed())
			writeFile(filepath.Join(clone, ".gitattributes"), "*.go filter=plant\n")
			writeFile(filepath.Join(clone, "plant.go"), "package main\n")
			runGit(clone, "add", "-A")
			runGit(clone, "commit", "--quiet", "-m", "attributes")
			sha := runGit(clone, "rev-parse", "HEAD")
			runGit(clone, "config", "filter.plant.smudge", script)
			runGit(clone, "config", "filter.plant.required", "true")
			runGit(clone, "config", "core.hooksPath", hooks)
			home := GinkgoT().TempDir()
			writeFile(filepath.Join(home, ".gitconfig"),
				"[filter \"plant\"]\n\tsmudge = "+script+"\n\trequired = true\n[core]\n\thooksPath = "+hooks+"\n")
			GinkgoT().Setenv("HOME", home)
			GinkgoT().Setenv("GIT_CONFIG_COUNT", "2")
			GinkgoT().Setenv("GIT_CONFIG_KEY_0", "filter.plant.smudge")
			GinkgoT().Setenv("GIT_CONFIG_VALUE_0", script)
			GinkgoT().Setenv("GIT_CONFIG_KEY_1", "core.hooksPath")
			GinkgoT().Setenv("GIT_CONFIG_VALUE_1", hooks)
			reference := validReference()
			reference.SHA = sha
			dir := filepath.Join(GinkgoT().TempDir(), "checkout")

			Expect(evalpatterns.CheckoutReference(reference, clone, dir)).To(Succeed())

			Expect(filepath.Join(planted, "ran")).NotTo(BeAnExistingFile())
			Expect(os.ReadFile(filepath.Join(dir, "plant.go"))).To(Equal([]byte("package main\n")))
		})
	})
})
