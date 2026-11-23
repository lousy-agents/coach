package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// ADR-009 reads the committed copy only for disclosure, so a valid policy on disk is used whatever HEAD commits at its path.
var _ = Describe("coach codesignal reads the project policy from disk: a valid policy on disk over a committed policy that is not usable (#168 A2b)", func() {
	const customPolicyPath = "policies/custom.json"

	type committedState struct {
		description string
		commit      func(path string) (repo, base string)
	}
	committedAs := func(body string) func(path string) (string, string) {
		return func(path string) (string, string) { return diskPolicyRepo(scopePolicyFile{path, body}) }
	}
	asSymbolicLink := func(path string) (string, string) {
		repo, base := diskPolicyRepo(scopePolicyFile{"real.json", singleRootPolicyJSON})
		symlinkAt(repo, path)
		gitIn(repo, "add", path)
		gitInWithIdentity(repo, "commit", "-q", "-m", "the policy as a link")
		base = scopePolicyHead(repo)
		commitFile(repo, "c.go", "package a\n\nfunc C() {}\n")
		Expect(runGitInRepo(repo, "ls-tree", "HEAD", path)).To(HavePrefix("120000 "), "fixture: the committed policy is a symbolic link")
		Expect(os.Remove(filepath.Join(repo, path))).To(Succeed())
		return repo, base
	}
	asDirectory := func(path string) (string, string) {
		repo, base := diskPolicyRepo(scopePolicyFile{path + "/inner.json", singleRootPolicyJSON})
		Expect(runGitInRepo(repo, "ls-tree", "HEAD", path)).To(HavePrefix("040000 "), "fixture: the committed policy is a directory")
		Expect(os.RemoveAll(filepath.Join(repo, path))).To(Succeed())
		return repo, base
	}
	states := []committedState{
		{"invalid JSON", committedAs("{bad")},
		{"valid JSON that the schema rejects", committedAs(invalidPolicy)},
		{"a symbolic link", asSymbolicLink},
		{"a directory", asDirectory},
	}

	type target struct {
		description string
		path        string
		args        []string
	}
	targets := []target{
		{"a discovered canonical policy", coachConfigPath, nil},
		{"the canonical policy named with --project-config", coachConfigPath, []string{"--project-config", coachConfigPath}},
		{"a named policy", customPolicyPath, []string{"--project-config", customPolicyPath}},
	}

	for _, mode := range scopePolicyModes {
		for _, state := range states {
			for _, subject := range targets {
				It("#168 A2b: "+subject.description+" committed as "+state.description+" and fixed on disk is used under "+mode.name+": exit 0, differs_from_head, the findings are the fix's", func() {
					repo, base := state.commit(subject.path)
					scopePolicyRewrite(repo, subject.path, goLayerPolicyConfigJSON)

					report, jsonOut := diskPolicyJSONScan(mode, repo, base, subject.args...)

					Expect(diskPolicyLayerViolations(report)).To(HaveLen(1), "the findings must use the policy on disk")
					Expect(scopePolicyScopeOf(jsonOut)).To(HaveKeyWithValue("policy",
						Equal(diskPolicyScope(subject.path, goLayerPolicyConfigJSON, diskCauseDiffersFromHead))))
					text := diskPolicyTextScan(mode, repo, base, subject.args...)
					expectDiskPolicyDisclosure(mode, text, diskPolicyLine(subject.path, goLayerPolicyConfigJSON, "differs from HEAD."), diskPolicyDiffersWarning)
				})
			}
		}

		It("#168 A2b: a canonical policy committed as invalid JSON and fixed on disk to a policy with no violation qualifies the all-clear under "+mode.name, func() {
			repo, base := committedAs("{bad")(coachConfigPath)
			scopePolicyRewrite(repo, coachConfigPath, diskPolicyWithoutForbiddenImports)

			text := diskPolicyTextScan(mode, repo, base)

			expectDiskPolicyDisclosure(mode, text, diskPolicyLine(coachConfigPath, diskPolicyWithoutForbiddenImports, "differs from HEAD."), diskPolicyDiffersWarning)
			Expect(string(text)).To(ContainSubstring("\n" + diskPolicyQualifiedAllClear + "\n"))
		})

		It("#168 A2b: a canonical policy committed as invalid JSON and fixed on disk with the fix staged is used under "+mode.name+" as well", func() {
			repo, base := committedAs("{bad")(coachConfigPath)
			scopePolicyRewrite(repo, coachConfigPath, goLayerPolicyConfigJSON)
			gitIn(repo, "add", coachConfigPath)
			expectGitStatusContains(repo, "M  "+coachConfigPath)

			report, jsonOut := diskPolicyJSONScan(mode, repo, base)

			Expect(diskPolicyLayerViolations(report)).To(HaveLen(1))
			Expect(scopePolicyScopeOf(jsonOut)).To(HaveKeyWithValue("policy",
				Equal(diskPolicyScope(coachConfigPath, goLayerPolicyConfigJSON, diskCauseDiffersFromHead))))
		})
	}
})
