package main_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// taskCoachBuild is the line of a mise task that builds coach (not coach-eval) from this checkout.
func taskCoachBuild(task string) string {
	GinkgoHelper()
	config, err := os.ReadFile(filepath.Join(repositoryRoot(), "mise.toml"))
	Expect(err).NotTo(HaveOccurred())
	body := regexp.MustCompile(`(?s)\[tasks\.` + regexp.QuoteMeta(task) + `\](.*?)(\n\[|\z)`).FindSubmatch(config)
	Expect(body).NotTo(BeNil(), "mise.toml defines [tasks.%s]", task)
	line := regexp.MustCompile(`(?m)^go build .*\./cmd/coach$`).Find(body[1])
	Expect(line).NotTo(BeNil(), "[tasks.%s] builds coach", task)
	return string(line)
}

// checkoutOfHead clones this repository's HEAD into a fresh directory, so that two clones of one commit sit at two
// different paths, as two worktrees or two runs' checkouts do.
func checkoutOfHead() string {
	GinkgoHelper()
	dir := filepath.Join(GinkgoT().TempDir(), "checkout")
	clone := exec.Command("git", "-c", "core.hooksPath=/dev/null", "clone", "--quiet", "--no-hardlinks", repositoryRoot(), dir)
	output, err := clone.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), string(output))
	return dir
}

// runTaskBuild runs a task's coach build line in checkout, with the task's bin_dir, and returns the binary's path.
func runTaskBuild(line, checkout string) string {
	GinkgoHelper()
	binDir := GinkgoT().TempDir()
	build := exec.Command("sh", "-c", `set -eu; bin_dir="$1"; `+line, "sh", binDir)
	build.Dir = checkout
	output, err := build.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), string(output))
	return filepath.Join(binDir, "coach")
}

var _ = Describe("the eval tasks' coach build (#881)", func() {
	DescribeTable("builds byte-identical binaries from two checkouts of one commit, so source.ref is stable",
		func(task string) {
			line := taskCoachBuild(task)

			first, err := os.ReadFile(runTaskBuild(line, checkoutOfHead()))
			Expect(err).NotTo(HaveOccurred())
			second, err := os.ReadFile(runTaskBuild(line, checkoutOfHead()))
			Expect(err).NotTo(HaveOccurred())

			Expect(bytes.Equal(first, second)).To(BeTrue(), "%s: %q built different binaries from two checkouts", task, line)
		},
		Entry("eval-census", "eval-census"),
		Entry("eval-history", "eval-history"),
		Entry("eval-antipatterns", "eval-antipatterns"),
	)

	DescribeTable("builds the same binary from a checkout before and after a tracked file is edited, as a census run leaves it",
		func(task string) {
			line := taskCoachBuild(task)
			checkout := checkoutOfHead()

			clean, err := os.ReadFile(runTaskBuild(line, checkout))
			Expect(err).NotTo(HaveOccurred())
			edited := filepath.Join(checkout, "README.md")
			content, err := os.ReadFile(edited)
			Expect(err).NotTo(HaveOccurred())
			Expect(os.WriteFile(edited, append(content, "\nan uncommitted edit\n"...), 0o644)).To(Succeed())
			dirty, err := os.ReadFile(runTaskBuild(line, checkout))
			Expect(err).NotTo(HaveOccurred())

			Expect(bytes.Equal(clean, dirty)).To(BeTrue(), "%s: %q stamped the working tree's state into the binary", task, line)
		},
		Entry("eval-census", "eval-census"),
		Entry("eval-history", "eval-history"),
		Entry("eval-antipatterns", "eval-antipatterns"),
	)

	It("builds coach with -trimpath -buildvcs=false in every task that builds it into the task's bin_dir", func() {
		config, err := os.ReadFile(filepath.Join(repositoryRoot(), "mise.toml"))
		Expect(err).NotTo(HaveOccurred())
		lines := regexp.MustCompile(`(?m)^go build .*-o "\$bin_dir/coach" \./cmd/coach$`).FindAll(config, -1)

		Expect(lines).NotTo(BeEmpty())
		for _, line := range lines {
			Expect(strings.Fields(string(line))).To(ContainElements("-trimpath", "-buildvcs=false"), "%s", line)
		}
	})

	It("gives a census re-run at the same corpus SHA the same record_id content prefix and source.ref", func() {
		line := taskCoachBuild("eval-census")
		busyPath, busySHA := censusRepo(goRepoFiles(complexFunctions(2)))
		corpus := writeCorpus(corpusRepo{ID: "busy", Path: busyPath, SHA: busySHA, Language: "go"})

		identities := func(coach string) []string {
			outcome, _, outputs := runCensusInto(GinkgoT().TempDir(), corpus, coach)
			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			var ids []string
			for _, record := range jsonLines(filepath.Join(outputs.censusRoot, "fixture", "records.jsonl")) {
				source := record["source"].(map[string]any)
				contentPrefix, _, found := strings.Cut(record["record_id"].(string), "-")
				Expect(found).To(BeTrue(), "record_id %q has a content prefix and a random suffix", record["record_id"])
				ids = append(ids, contentPrefix+" "+source["ref"].(string))
			}
			Expect(ids).NotTo(BeEmpty())
			return ids
		}

		Expect(identities(runTaskBuild(line, checkoutOfHead()))).To(Equal(identities(runTaskBuild(line, checkoutOfHead()))))
	})
})
