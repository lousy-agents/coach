package main_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

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

	When("Coach reports the project coverage incomplete", func() {
		It("records analysis_failed and exits 3", func() {
			path, sha := censusRepo(map[string]string{
				"go.mod":                   "module example.com/fx\n\ngo 1.26.0\n",
				"pkg/app/a.go":             "package app\n\nfunc A() {}\n",
				".lousy-agents/coach.json": `{"schema_version":"1","roots":["."],"layers":[{"name":"usecases","prefixes":["pkg/usecases"]},{"name":"app","prefixes":["pkg/app"]}],"forbidden_imports":[],"required_layer":"usecases"}`,
			})
			corpus := writeCorpus(corpusRepo{ID: "partial", Path: path, SHA: sha, Language: "go"})

			outcome, summary := runCensus(corpus, coachBinary())

			Expect(outcome.exitCode).To(Equal(3), outcome.stderr)
			Expect(summary.status("partial").Status).To(Equal("analysis_failed"))
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

	When("a Go repository comes before a TypeScript one", func() {
		It("still exits 2, naming the TypeScript repository, and scans nothing", func() {
			goPath, goSHA := censusRepo(goRepoFiles(complexFunctions(1)))
			tsPath, tsSHA := censusRepo(map[string]string{"src/a.ts": "export const A = 1;\n"})
			corpus := writeCorpus(corpusRepo{ID: "api", Path: goPath, SHA: goSHA, Language: "go"}, corpusRepo{ID: "web", Path: tsPath, SHA: tsSHA, Language: "typescript"})

			outcome, _ := runCensus(corpus, coachBinary())

			Expect(outcome.exitCode).To(Equal(2))
			Expect(outcome.stderr).To(ContainSubstring(`"web"`))
			Expect(outcome.stdout).To(BeEmpty())
		})
	})

	When("the coach binary exists but cannot run", func() {
		It("records analysis_failed and exits 3", func() {
			path, sha := censusRepo(goRepoFiles(complexFunctions(1)))
			corpus := writeCorpus(corpusRepo{ID: "busy", Path: path, SHA: sha, Language: "go"})
			notExecutable := filepath.Join(GinkgoT().TempDir(), "coach")
			Expect(os.WriteFile(notExecutable, []byte("not a program"), 0o644)).To(Succeed())

			outcome, summary := runCensus(corpus, notExecutable)

			Expect(outcome.exitCode).To(Equal(3), outcome.stderr)
			Expect(summary.status("busy").Status).To(Equal("analysis_failed"))
		})
	})

	When("the coach binary does not exist", func() {
		It("exits 2 naming --coach, before scanning anything", func() {
			path, sha := censusRepo(goRepoFiles(complexFunctions(1)))
			corpus := writeCorpus(corpusRepo{ID: "busy", Path: path, SHA: sha, Language: "go"})

			outcome, _ := runCensus(corpus, filepath.Join(GinkgoT().TempDir(), "no-such-coach"))

			Expect(outcome.exitCode).To(Equal(2))
			Expect(outcome.stderr).To(ContainSubstring("--coach"))
			Expect(outcome.stdout).To(BeEmpty())
		})
	})

	When("coach exits 0 but its report cannot be read", func() {
		It("records analysis_failed and exits 3, because the census is not clean", func() {
			path, sha := censusRepo(goRepoFiles(complexFunctions(1)))
			corpus := writeCorpus(corpusRepo{ID: "busy", Path: path, SHA: sha, Language: "go"})
			garbled := filepath.Join(GinkgoT().TempDir(), "coach")
			Expect(os.WriteFile(garbled, []byte("#!/bin/sh\necho 'not a report'\n"), 0o755)).To(Succeed())

			outcome, summary := runCensus(corpus, garbled)

			Expect(outcome.exitCode).To(Equal(3), outcome.stderr)
			Expect(summary.status("busy").Status).To(Equal("analysis_failed"))
		})
	})

	When("a census finishes", func() {
		It("leaves no clone behind in the temporary directory", func() {
			path, sha := censusRepo(goRepoFiles(complexFunctions(1)))
			corpus := writeCorpus(corpusRepo{ID: "busy", Path: path, SHA: sha, Language: "go"})
			tmp := GinkgoT().TempDir()

			outcome, _ := runCensus(corpus, coachBinary(), "TMPDIR="+tmp)

			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			Expect(filepath.Glob(filepath.Join(tmp, "coach-eval-census*"))).To(BeEmpty())
		})
	})

	When("the coach binary is named by a path relative to where the census runs", func() {
		It("runs that binary with exactly the scan arguments, and records its absolute path", func() {
			path, sha := censusRepo(goRepoFiles(complexFunctions(1)))
			corpus := writeCorpus(corpusRepo{ID: "busy", Path: path, SHA: sha, Language: "go", ProjectConfig: ".lousy-agents/coach.json"})
			binDir := GinkgoT().TempDir()
			argsFile := filepath.Join(binDir, "args")
			fake := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + argsFile + "\necho '{\"schema_version\":\"1\",\"signals\":[]}'\n"
			Expect(os.WriteFile(filepath.Join(binDir, "coach"), []byte(fake), 0o755)).To(Succeed())

			outcome := runProgram(binDir, nil, coachEvalBinary, "census", "--coach", "./coach", corpus)

			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			Expect(readArgs(argsFile)).To(Equal([]string{"codesignal", "--baseline", "--format", "json", "--no-interactive", "--fail-on-incomplete-coverage", "--project-config", ".lousy-agents/coach.json"}))
			var summary censusSummary
			Expect(json.Unmarshal([]byte(outcome.stdout), &summary)).To(Succeed())
			Expect(filepath.IsAbs(summary.Coach.Path)).To(BeTrue(), summary.Coach.Path)
		})
	})

	When("the census input is not usable", func() {
		DescribeTable("it exits 2, naming the problem",
			func(problem string, args ...string) {
				outcome := runProgram("", nil, coachEvalBinary, args...)
				Expect(outcome.exitCode).To(Equal(2))
				Expect(outcome.stderr).To(ContainSubstring(problem))
			},
			Entry("no corpus manifest", "corpus manifest", "census", "--coach", "/bin/true"),
			Entry("no coach binary", "--coach", "census", "/nonexistent/corpus.json"),
			Entry("a corpus manifest that does not load", "/nonexistent/corpus.json", "census", "--coach", "/bin/true", "/nonexistent/corpus.json"),
			Entry("two corpus manifests", "exactly one corpus manifest", "census", "--coach", "/bin/true", "a.json", "b.json"),
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

	DescribeTable("a filter in the caller's global git configuration is never run",
		func(globalEnv func(markers string) string) {
			markers := GinkgoT().TempDir()
			Expect(os.WriteFile(filepath.Join(markers, ".gitconfig"), []byte("[filter \"planted\"]\n\tsmudge = touch "+filepath.Join(markers, "smudge")+"; cat\n"), 0o644)).To(Succeed())
			path, sha := censusRepo(map[string]string{
				"go.mod":         "module example.com/fx\n\ngo 1.26.0\n",
				"p/p.go":         complexFunctions(1),
				".gitattributes": "* filter=planted\n",
			})
			corpus := writeCorpus(corpusRepo{ID: "planted", Path: path, SHA: sha, Language: "go"})

			outcome, _ := runCensus(corpus, coachBinary(), globalEnv(markers))

			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			Expect(filepath.Join(markers, "smudge")).NotTo(BeAnExistingFile())
		},
		Entry("named by GIT_CONFIG_GLOBAL", func(markers string) string { return "GIT_CONFIG_GLOBAL=" + filepath.Join(markers, ".gitconfig") }),
		Entry("found through HOME", func(markers string) string { return "HOME=" + markers }),
	)
})

func readArgs(path string) []string {
	raw, err := os.ReadFile(path)
	Expect(err).NotTo(HaveOccurred(), "the coach binary never ran")
	return strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
}
