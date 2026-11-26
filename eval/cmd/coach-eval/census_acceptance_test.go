package main_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach-eval census", func() {
	When("one repository has 12 findings of a rule and another has none", func() {
		It("records measured with 12 findings and not_exercised, and exits 0", func() {
			busyPath, busySHA := censusRepo(goRepoFiles(complexFunctions(12)))
			quietPath, quietSHA := censusRepo(goRepoFiles(complexFunctions(0)))
			corpus := writeCorpus(corpusRepo{ID: "busy", Path: busyPath, SHA: busySHA, Language: "go"}, corpusRepo{ID: "quiet", Path: quietPath, SHA: quietSHA, Language: "go"})

			outcome, summary := runCensus(corpus, coachBinary())

			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			Expect(summary.status("busy")).To(Equal(censusStatus{RuleID: "complexity.cognitive_complexity", Status: "measured", Findings: 12, Adjudicable: 12}))
			Expect(summary.status("quiet").Status).To(Equal("not_exercised"))
		})

		It("records the SHA of each repository and of the coach binary", func() {
			busyPath, busySHA := censusRepo(goRepoFiles(complexFunctions(1)))
			corpus := writeCorpus(corpusRepo{ID: "busy", Path: busyPath, SHA: busySHA, Language: "go"})

			_, summary := runCensus(corpus, coachBinary())

			raw, err := os.ReadFile(coachBinary())
			Expect(err).NotTo(HaveOccurred())
			digest := sha256.Sum256(raw)
			Expect(summary.Coach.SHA256).To(Equal(hex.EncodeToString(digest[:])))
			Expect(summary.Repositories[0].SHA).To(Equal(busySHA))
		})
	})

	When("coach fails on one repository", func() {
		It("records analysis_failed with coach's stderr, still scans the others, and exits 3", func() {
			brokenPath, brokenSHA := censusRepo(goRepoFiles(complexFunctions(1)))
			busyPath, busySHA := censusRepo(goRepoFiles(complexFunctions(2)))
			corpus := writeCorpus(
				corpusRepo{ID: "broken", Path: brokenPath, SHA: brokenSHA, Language: "go", ProjectConfig: ".lousy-agents/missing.json"},
				corpusRepo{ID: "busy", Path: busyPath, SHA: busySHA, Language: "go"},
			)

			outcome, summary := runCensus(corpus, coachBinary())

			Expect(outcome.exitCode).To(Equal(3), outcome.stderr)
			Expect(summary.status("broken").Status).To(Equal("analysis_failed"))
			Expect(summary.status("broken").Evidence).To(ContainSubstring("missing.json"))
			Expect(summary.status("busy").Findings).To(Equal(2))
		})
	})

	When("a mutated threshold is in force", func() {
		It("no longer reports the 12 measured findings, so the expectation above turns red", func() {
			busyPath, busySHA := censusRepo(goRepoFiles(complexFunctions(12)))
			corpus := writeCorpus(corpusRepo{ID: "busy", Path: busyPath, SHA: busySHA, Language: "go"})

			outcome, summary := runCensus(corpus, coachOverrideBinary(), `COACH_EVAL_OVERRIDE={"thresholds":{"complexity.cognitive_complexity":100}}`)

			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			Expect(summary.status("busy")).NotTo(Equal(censusStatus{RuleID: "complexity.cognitive_complexity", Status: "measured", Findings: 12, Adjudicable: 12}))
			Expect(summary.status("busy").Status).To(Equal("not_exercised"))
		})
	})

	When("a repository's SHA is not in that repository", func() {
		It("exits 2, naming the repository and the SHA, before scanning anything", func() {
			path, _ := censusRepo(goRepoFiles(complexFunctions(1)))
			missing := "0123456789abcdef0123456789abcdef01234567"
			corpus := writeCorpus(corpusRepo{ID: "gone", Path: path, SHA: missing, Language: "go"})

			outcome, _ := runCensus(corpus, coachBinary())

			Expect(outcome.exitCode).To(Equal(2))
			Expect(outcome.stderr).To(And(ContainSubstring("gone"), ContainSubstring(missing)))
			Expect(outcome.stdout).To(BeEmpty())
		})
	})

	When("the corpus holds a TypeScript repository", func() {
		It("exits 2, naming the reason and the pinned-image follow-up", func() {
			path, sha := censusRepo(map[string]string{"src/a.ts": "export const A = 1;\n"})
			corpus := writeCorpus(corpusRepo{ID: "web", Path: path, SHA: sha, Language: "typescript"})

			outcome, _ := runCensus(corpus, coachBinary())

			Expect(outcome.exitCode).To(Equal(2))
			Expect(outcome.stderr).To(And(ContainSubstring("web"), ContainSubstring("#832")))
		})
	})

	When("the census input is not usable", func() {
		DescribeTable("it exits 2, naming the problem",
			func(problem string, args ...string) {
				outcome := runCmd(commandFor(args...))
				Expect(outcome.exitCode).To(Equal(2))
				Expect(outcome.stderr).To(ContainSubstring(problem))
			},
			Entry("no corpus manifest", "corpus manifest", "census", "--coach", "/bin/true"),
			Entry("no coach binary", "--coach", "census", "/nonexistent/corpus.json"),
			Entry("a corpus manifest that does not load", "/nonexistent/corpus.json", "census", "--coach", "/bin/true", "/nonexistent/corpus.json"),
		)
	})

	When("a corpus repository defines a smudge filter and a post-checkout hook", func() {
		It("runs neither: the census reads no repository-supplied configuration or hook", func() {
			markers := GinkgoT().TempDir()
			path, sha := censusRepo(map[string]string{
				"go.mod":         "module example.com/fx\n\ngo 1.26.0\n",
				"p/p.go":         complexFunctions(1),
				".gitattributes": "* filter=planted\n",
			})
			isolatedGit(path, "config", "filter.planted.smudge", "touch "+filepath.Join(markers, "smudge")+"; cat")
			hook := filepath.Join(path, ".git", "hooks", "post-checkout")
			Expect(os.WriteFile(hook, []byte("#!/bin/sh\ntouch "+filepath.Join(markers, "hook")+"\n"), 0o755)).To(Succeed())
			corpus := writeCorpus(corpusRepo{ID: "planted", Path: path, SHA: sha, Language: "go"})

			outcome, summary := runCensus(corpus, coachBinary())

			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			Expect(summary.status("planted").Findings).To(Equal(1))
			Expect(filepath.Join(markers, "smudge")).NotTo(BeAnExistingFile())
			Expect(filepath.Join(markers, "hook")).NotTo(BeAnExistingFile())
		})
	})

	When("the caller's global git configuration defines a filter a repository selects", func() {
		It("does not run it", func() {
			markers := GinkgoT().TempDir()
			global := filepath.Join(markers, "gitconfig")
			Expect(os.WriteFile(global, []byte("[filter \"planted\"]\n\tsmudge = touch "+filepath.Join(markers, "smudge")+"; cat\n"), 0o644)).To(Succeed())
			path, sha := censusRepo(map[string]string{
				"go.mod":         "module example.com/fx\n\ngo 1.26.0\n",
				"p/p.go":         complexFunctions(1),
				".gitattributes": "* filter=planted\n",
			})
			corpus := writeCorpus(corpusRepo{ID: "planted", Path: path, SHA: sha, Language: "go"})

			outcome, _ := runCensus(corpus, coachBinary(), "GIT_CONFIG_GLOBAL="+global)

			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			Expect(filepath.Join(markers, "smudge")).NotTo(BeAnExistingFile())
		})
	})
})
