package main

import (
	"os"
	"path/filepath"
	"strings"
	"unicode"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	renameOnDisk          = "rename the file on disk"
	passTheSpellingOnDisk = "pass the spelling on disk"
)

// spellingTarget is one way a scan is told which policy to read, with the spellings of that path that differ from it
// only in case: the file's name, a directory's name, and both.
type spellingTarget struct {
	name                          string
	args                          []string
	policy                        string
	fileCase, directoryCase, both string
}

var spellingTargets = []spellingTarget{
	{"the policy Coach finds", nil, coachConfigPath, ".lousy-agents/Coach.json", ".Lousy-Agents/coach.json", ".Lousy-Agents/Coach.json"},
	{"the default path named with --project-config", []string{"--project-config", coachConfigPath}, coachConfigPath, ".lousy-agents/Coach.json", ".Lousy-Agents/coach.json", ".Lousy-Agents/Coach.json"},
	{"another path named with --project-config", []string{"--project-config", "cfg/p.json"}, "cfg/p.json", "cfg/P.json", "CFG/p.json", "CFG/P.json"},
}

// caseVariantsOf is count distinct names that differ from name only by case.
func caseVariantsOf(name string, count int) []string {
	variants := []string{}
	seen := map[string]bool{name: true}
	for mask := 1; len(variants) < count; mask++ {
		runes := []rune(name)
		for i := range runes {
			if mask&(1<<i) != 0 {
				runes[i] = unicode.ToUpper(runes[i])
			}
		}
		if variant := string(runes); !seen[variant] {
			seen[variant] = true
			variants = append(variants, variant)
		}
	}
	return variants
}

// scanJSONRefused runs one scan to the refusal the disk spelling walk gives and returns its standard error.
func scanJSONRefused(mode scopePolicyMode, repo, base string, target spellingTarget) string {
	GinkgoHelper()
	args := append(append([]string{}, target.args...), "--format=json", "--no-interactive")
	stdout, stderr, exitCode := mode.run(repo, base, args...)
	Expect(exitCode).To(Equal(2), "stdout: %s\nstderr: %s", stdout, stderr)
	Expect(string(stdout)).To(BeEmpty(), "no report beside a refusal")
	return string(stderr)
}

func expectTheDiskSpellingRefusal(stderr string) {
	GinkgoHelper()
	Expect(stderr).To(ContainSubstring("project_config_invalid"), "stderr:\n%s", stderr)
	Expect(stderr).To(ContainSubstring("(cause=name_case_differs)"), "stderr:\n%s", stderr)
	Expect(stderr).To(ContainSubstring(renameOnDisk), "stderr:\n%s", stderr)
	Expect(stderr).To(ContainSubstring(passTheSpellingOnDisk), "stderr:\n%s", stderr)
	Expect(stderr).NotTo(ContainSubstring("project_config_missing"), "a case-only difference is never a missing policy:\n%s", stderr)
	Expect(strings.ToLower(stderr)).NotTo(ContainSubstring("commit"), "the disk decides the spelling, so nothing is to be committed:\n%s", stderr)
	expectNoRunnableCommand(stderr)
}

// The disk decides the spelling (ADR-009 decision 5): the path is compared with the directory listing case-folded, on
// every file system, so a case-only difference is refused the same on a disk that folds case and on one that does not,
// whatever the revision commits.
var _ = Describe("coach codesignal: the spelling of the policy path is the one the disk lists", func() {
	for _, mode := range scopePolicyModes {
		for _, target := range spellingTargets {
			When("scanning "+mode.name+" with "+target.name, func() {
				DescribeTable("refuses a policy that exists on disk only in another case, naming the way out on disk",
					func(variant func(spellingTarget) string) {
						repo, base := scopePolicyRepo()
						writeInRepo(repo, variant(target), singleRootPolicyJSON)

						stderr := scanJSONRefused(mode, repo, base, target)

						expectTheDiskSpellingRefusal(stderr)
						Expect(stderr).To(ContainSubstring(`the policy is on disk as "`+variant(target)+`", but Coach reads "`+target.policy+`", so it is not read: `), "stderr:\n%s", stderr)
					},
					Entry("the file's name differs", func(t spellingTarget) string { return t.fileCase }),
					Entry("a directory's name differs", func(t spellingTarget) string { return t.directoryCase }),
					Entry("a directory's and the file's names differ", func(t spellingTarget) string { return t.both }),
				)

				It("refuses a spelling the revision commits as well, because the disk and not the revision decides", func() {
					repo, base := scopePolicyRepo(scopePolicyFile{target.fileCase, singleRootPolicyJSON})

					stderr := scanJSONRefused(mode, repo, base, target)

					expectTheDiskSpellingRefusal(stderr)
				})

				It("reads the entry spelled exactly as the path, and not the case variants beside it", func() {
					repo, base := scopePolicyRepo()
					writeInRepo(repo, target.fileCase, srcRootPolicyJSON)
					writeInRepo(repo, target.directoryCase, srcRootPolicyJSON)
					writeInRepo(repo, target.policy, singleRootPolicyJSON)

					jsonOut, stderr, exitCode := mode.run(repo, base, append(append([]string{}, target.args...), "--format=json", "--no-interactive")...)

					Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
					Expect(scopePolicyScopeOf(jsonOut)).To(HaveKeyWithValue("policy", Equal(diskPolicyScope(target.policy, singleRootPolicyJSON, diskCauseNotCommitted))))
				})

				DescribeTable("lists the case variants on disk when none is spelled exactly as the path, up to 16, and chooses none past 16",
					func(count int, wantsAChoice bool) {
						repo, base := scopePolicyRepo()
						directory, file := filepath.Split(target.policy)
						variants := caseVariantsOf(file, count)
						for _, variant := range variants {
							writeInRepo(repo, directory+variant, singleRootPolicyJSON)
						}

						stderr := scanJSONRefused(mode, repo, base, target)

						expectTheDiskSpellingRefusal(stderr)
						Expect(stderr).To(ContainSubstring("several spellings"), "stderr:\n%s", stderr)
						Expect(stderr).To(ContainSubstring(`and Coach reads "`+target.policy+`"`), "stderr:\n%s", stderr)
						named := 0
						for _, variant := range variants {
							if strings.Contains(stderr, `"`+directory+variant+`"`) {
								named++
							}
						}
						if !wantsAChoice {
							Expect(stderr).To(ContainSubstring(", and more, and Coach reads"), "past 16 the message says there are more:\n%s", stderr)
							Expect(stderr).NotTo(ContainSubstring("To keep"), "past 16 no spelling is chosen:\n%s", stderr)
							Expect(named).To(Equal(16), "past 16 exactly 16 are named:\n%s", stderr)
						} else {
							Expect(stderr).NotTo(ContainSubstring("and more"), "stderr:\n%s", stderr)
							Expect(stderr).To(ContainSubstring("To keep"), "up to 16 one is chosen to keep:\n%s", stderr)
							Expect(named).To(Equal(len(variants)), "every variant is named:\n%s", stderr)
						}
					},
					Entry("two variants", 2, true),
					Entry("sixteen variants", 16, true),
					Entry("seventeen variants", 17, false),
				)
			})
		}
	}
})

// A spelling that differs only from the one committed at HEAD is disclosed and not refused, so an uncommitted rename
// that fixes the case works before the commit.
var _ = Describe("coach codesignal: a spelling on disk that differs from the committed one is disclosed, not refused", func() {
	const notCommittedSuffix = "not committed at HEAD."
	moves := []struct {
		name string
		move func(repo, from, to string)
	}{
		{"a rename staged with git mv", renameInTheRepository},
		{"a rename git has not been told of", func(repo, from, to string) {
			Expect(os.MkdirAll(filepath.Join(repo, filepath.Dir(to)), 0o755)).To(Succeed())
			Expect(os.Rename(filepath.Join(repo, from), filepath.Join(repo, to))).To(Succeed())
		}},
	}
	for _, mode := range scopePolicyModes {
		for _, target := range []spellingTarget{spellingTargets[0], spellingTargets[2]} {
			for _, move := range moves {
				for _, committed := range []string{target.fileCase, target.directoryCase} {
					It("reads "+target.policy+" under "+mode.name+" when HEAD commits "+committed+" and the policy is on disk as the path says, after "+move.name+", with matches_head false and a WARNING", func() {
						repo, base := scopePolicyRepo(scopePolicyFile{committed, singleRootPolicyJSON})
						move.move(repo, committed, target.policy)
						jsonArgs := append(append([]string{}, target.args...), "--format=json", "--no-interactive")

						jsonOut, stderr, exitCode := mode.run(repo, base, jsonArgs...)

						Expect(exitCode).To(Equal(0), "the disk spelling is the path, so nothing is refused; stderr: %s", stderr)
						Expect(scopePolicyScopeOf(jsonOut)).To(HaveKeyWithValue("policy", Equal(diskPolicyScope(target.policy, singleRootPolicyJSON, diskCauseNotCommitted))))
						text, stderr, exitCode := mode.run(repo, base, append(append([]string{}, target.args...), "--no-interactive")...)
						Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
						expectDiskPolicyDisclosure(mode, text, diskPolicyLine(target.policy, singleRootPolicyJSON, notCommittedSuffix), diskPolicyNotCommittedWarning(target.policy))
					})
				}
			}
		}
	}
})
