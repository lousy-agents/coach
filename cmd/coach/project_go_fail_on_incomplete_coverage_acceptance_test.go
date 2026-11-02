package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// junkFileCount is one more than the 5000 files the Go project backend admits
// (goProjectBudgets.MaxInputFiles). Root discovery counts every file in the
// snapshot, not only Go files, so this many files truncates discovery and the
// Go model is reported incomplete.
const junkFileCount = 5001

func runGitInRepo(repo string, args ...string) string {
	command := exec.Command("git", args...)
	command.Dir = repo
	command.Env = commitEnv
	output, err := command.CombinedOutput()
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "git %v: %s", args, output)
	return strings.TrimSpace(string(output))
}

// commitJunkFiles writes count empty files under junk/ and commits them with
// a single add and commit, because one commit per file would dominate the run.
func commitJunkFiles(repo string, count int) string {
	Expect(os.MkdirAll(filepath.Join(repo, "junk"), 0o755)).To(Succeed())
	for i := 0; i < count; i++ {
		Expect(os.WriteFile(filepath.Join(repo, "junk", fmt.Sprintf("f%05d.txt", i)), nil, 0o644)).To(Succeed())
	}
	runGitInRepo(repo, "add", "-A", "junk")
	runGitInRepo(repo, "commit", "-q", "-m", "add junk files")
	return runGitInRepo(repo, "rev-parse", "HEAD")
}

func goLayeredRepo() string {
	repo := newTempGitRepo()
	commitFile(repo, "go.mod", goModuleFile)
	commitFile(repo, "pkg/db/db.go", dbPackageFile)
	commitFile(repo, "pkg/handlers/handlers.go", handlersWithoutImport)
	commitFile(repo, "project.json", goLayerPolicyConfigJSON)
	return repo
}

var _ = Describe("coach codesignal --fail-on-incomplete-coverage with the Go project backend", func() {
	When("the Go model is incomplete because discovery hit the input-file budget", Ordered, func() {
		var repo string

		BeforeAll(func() {
			repo = goLayeredRepo()
			commitJunkFiles(repo, junkFileCount)
		})

		It("exits 3 and still writes the report, whose project_coverage says why", func() {
			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo,
				"--project-config", "project.json", "--format=json", "--fail-on-incomplete-coverage")

			report := decodeCoachReport(stdout)
			Expect(report.ProjectCoverage).NotTo(BeNil(), "stdout: %.400s", stdout)
			Expect(report.ProjectCoverage.Complete).To(BeFalse(),
				"the fixture must make the model incomplete, or exit 0 would prove nothing")
			Expect(exitCode).To(Equal(3), "stderr: %s", stderr)
		})

		It("still exits 0 without the flag, because coach is advisory by default", func() {
			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo,
				"--project-config", "project.json", "--format=json")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(decodeCoachReport(stdout).ProjectCoverage.Complete).To(BeFalse())
		})
	})

	When("only the base side of a diff is incomplete", func() {
		It("exits 3 even though the head side is complete", func() {
			repo := goLayeredRepo()
			baseSHA := commitJunkFiles(repo, junkFileCount)
			runGitInRepo(repo, "rm", "-r", "-q", "junk")
			runGitInRepo(repo, "commit", "-q", "-m", "remove junk files")
			commitFile(repo, "pkg/handlers/handlers.go", handlersWithoutImport+"\n// edited\n")

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, baseSHA,
				"--project-config", "project.json", "--format=json", "--fail-on-incomplete-coverage")

			report := decodeCoachReport(stdout)
			Expect(report.ProjectCoverage).NotTo(BeNil(), "stdout: %.400s", stdout)
			Expect(report.ProjectCoverage.Complete).To(BeTrue(),
				"project_coverage is head-only; the base side must be what makes this exit 3")
			Expect(exitCode).To(Equal(3), "stderr: %s", stderr)
		})
	})

	When("the Go model is complete", func() {
		It("exits 0 with the flag, so the flag does not fail every Go run", func() {
			repo := goLayeredRepo()

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo,
				"--project-config", "project.json", "--format=json", "--fail-on-incomplete-coverage")

			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %.400s", stderr, stdout)
			Expect(decodeCoachReport(stdout).ProjectCoverage.Complete).To(BeTrue())
		})
	})
})
