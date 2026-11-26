package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// taskScript is the run script of a mise task, read from this checkout's mise.toml.
func taskScript(task string) string {
	GinkgoHelper()
	config, err := os.ReadFile(filepath.Join(repositoryRoot(), "mise.toml"))
	Expect(err).NotTo(HaveOccurred())
	script := regexp.MustCompile(`(?s)\[tasks\.` + regexp.QuoteMeta(task) + `\][^\[]*?run = """\n(.*?)"""`).FindSubmatch(config)
	Expect(script).NotTo(BeNil(), "mise.toml defines [tasks.%s] with a run script", task)
	return string(script[1])
}

var _ = Describe("the eval-patterns mise task (#872 Task 3)", func() {
	It("extracts the committed manifest's references from the clone given on the command line into the committed profiles (AC-1.1)", func() {
		checkout := checkoutOfHead()
		task := exec.Command("sh", "-c", taskScript("eval-patterns"), "sh", "--clone", "coach-self="+checkout)
		task.Dir = checkout

		outcome := runGenerateCommand(task)

		Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
		written, err := os.ReadFile(filepath.Join(checkout, "eval", "patterns", "profiles", "coach-self.profile.json"))
		Expect(err).NotTo(HaveOccurred())
		committed, err := os.ReadFile(filepath.Join(repositoryRoot(), "eval", "patterns", "profiles", "coach-self.profile.json"))
		Expect(err).NotTo(HaveOccurred())
		snapshot, err := os.ReadFile(filepath.Join(repositoryRoot(), "eval", "internal", "evalpatterns", "testdata", "coach-self.profile.json"))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(written)).To(Equal(string(committed)), "the committed profile is what the task writes")
		Expect(string(committed)).To(Equal(string(snapshot)), "the committed profile is the reviewed snapshot")
		Expect(isolatedGit(checkout, "status", "--porcelain")).To(BeEmpty(), "the task changes nothing but the committed profiles and derived fixtures, which it rewrites byte for byte")
		Expect(ids(filepath.Join(checkout, "eval", "fixtures", "antipatterns-derived", "coach-self"))).To(Equal([]string{"arrow-code", "god-file", "spaghetti-conditional"}))
	})

	It("commits derived fixtures that coach-eval antipatterns runs and reports, every one passing against the real coach (#872 Task 5)", func() {
		scan := runAntipatterns(filepath.Join(repositoryRoot(), "eval", "fixtures", "antipatterns-derived", "coach-self"), antipatternCoach())

		Expect(scan.exitCode).To(Equal(0), scan.stderr)
		Expect(coveredFixtureIDs(decodeTable(scan.stdout))).To(Equal(map[string]bool{"arrow-code": true, "god-file": true, "spaghetti-conditional": true}))
	})
})
