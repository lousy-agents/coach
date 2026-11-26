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
	)

	It("gives a census re-run at the same corpus SHA the same record_id and source.ref", func() {
		line := taskCoachBuild("eval-census")
		busyPath, busySHA := censusRepo(goRepoFiles(complexFunctions(2)))
		corpus := writeCorpus(corpusRepo{ID: "busy", Path: busyPath, SHA: busySHA, Language: "go"})

		identities := func(coach string) []string {
			outcome, _, outputs := runCensusInto(GinkgoT().TempDir(), corpus, coach)
			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			var ids []string
			for _, record := range jsonLines(filepath.Join(outputs.censusRoot, "fixture", "records.jsonl")) {
				source := record["source"].(map[string]any)
				ids = append(ids, strings.Join([]string{record["record_id"].(string), source["ref"].(string)}, " "))
			}
			Expect(ids).NotTo(BeEmpty())
			return ids
		}

		Expect(identities(runTaskBuild(line, checkoutOfHead()))).To(Equal(identities(runTaskBuild(line, checkoutOfHead()))))
	})
})
