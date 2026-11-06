package main

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// A policy gap that no remedy fixes says why, in a stable word, on the gap itself. Without it a caller
// of --check-project sees a bare policy_missing for a file that is on disk and only needs committing,
// and cannot tell that from a file that is gone. The text prints the same word from the same field, so
// the two never differ. Every word the CLI can be made to report has a row below, which scans the
// state with --check-project in JSON and in text.
type causeRow struct {
	name, policy, gap, cause string
	arrange                  func(repo string)
}

// unreachableCauses are the declared words that no input can make the CLI print, and why.
var unreachableCauses = map[string]string{
	"git_unreadable": "a git question that cannot be answered is an operational error (exit 1, no readiness result), so no gap carries it; only a read that trips the time or stderr bound produces it, and that needs git to stall for 30 s. The mapping is covered by unit tests.",
	"undetermined":   "the word for a verdict no offer knows; no state produces such a verdict, and the unit tests pin the default arm.",
	"invalid_path":   "a --project-config value that cannot name a policy is rejected as a usage error before any readiness check runs, so no gap carries the word; a scan's error does, and its spec is project_config_invalid_path_acceptance_test.go.",
}

func removeTree(repo, name string) {
	ExpectWithOffset(1, os.RemoveAll(filepath.Join(repo, name))).To(Succeed())
}

func gitWithInput(repo, input string, args ...string) string {
	command := exec.Command("git", args...)
	command.Dir = repo
	command.Stdin = strings.NewReader(input)
	output, err := command.CombinedOutput()
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "git %v: %s", args, output)
	return strings.TrimSpace(string(output))
}

func symlinkAt(repo, path string) {
	ExpectWithOffset(1, os.MkdirAll(filepath.Join(repo, filepath.Dir(path)), 0o755)).To(Succeed())
	ExpectWithOffset(1, os.Symlink("../real.json", filepath.Join(repo, path))).To(Succeed())
}

func stageThreeSidesOf(repo, path string) {
	blob := gitWithInput(repo, singleRootPolicyJSON, "hash-object", "-w", "--stdin")
	entries := "100644 " + blob + " 1\t" + path + "\n100644 " + blob + " 2\t" + path + "\n100644 " + blob + " 3\t" + path + "\n"
	gitWithInput(repo, entries, "update-index", "--index-info")
}

func commitThenRemove(repo, path string) {
	commitFile(repo, path, singleRootPolicyJSON)
	removeTree(repo, filepath.Dir(path))
}

var causeRows = []causeRow{
	{"a file on disk that is not committed", "un.json", "policy_missing", "not_committed", func(repo string) {
		writeWorktreeFile(repo, "un.json", singleRootPolicyJSON)
	}},
	{"a committed policy with a staged edit, then deleted", "cfg/p.json", "policy_missing", "staged_edit", func(repo string) {
		commitFile(repo, "cfg/p.json", singleRootPolicyJSON)
		writeWorktreeFile(repo, "cfg/p.json", `{"schema_version":"1","roots":["staged"]}`+"\n")
		gitIn(repo, "add", "cfg/p.json")
		removeTree(repo, "cfg")
	}},
	{"a file where the policy's directory should be", "cfg/p.json", "policy_missing", "parent_not_directory", func(repo string) {
		commitThenRemove(repo, "cfg/p.json")
		writeWorktreeFile(repo, "cfg", "a file in the way")
	}},
	{"files committed under the name, which makes it a directory", "cfg", "policy_missing", "directory", func(repo string) {
		commitThenRemove(repo, "cfg/p.json")
	}},
	{"a policy that is a symbolic link on disk", "link.json", "policy_invalid", "symbolic_link", func(repo string) {
		writeWorktreeFile(repo, "real.json", singleRootPolicyJSON)
		Expect(os.Symlink("real.json", filepath.Join(repo, "link.json"))).To(Succeed())
	}},
	{"a directory on disk where the policy file should be", "dir.json", "policy_invalid", "not_a_regular_file", func(repo string) {
		Expect(os.MkdirAll(filepath.Join(repo, "dir.json"), 0o755)).To(Succeed())
	}},
	{"a policy file the process cannot read", "locked.json", "policy_invalid", "unreadable_on_disk", func(repo string) {
		if os.Geteuid() == 0 {
			Skip("root can read a mode 000 file, so this state cannot be built")
		}
		writeWorktreeFile(repo, "locked.json", singleRootPolicyJSON)
		Expect(os.Chmod(filepath.Join(repo, "locked.json"), 0o000)).To(Succeed())
		DeferCleanup(func() { _ = os.Chmod(filepath.Join(repo, "locked.json"), 0o644) })
	}},
	{"a name spelled in another case than the revision holds, on a file system that folds case", "CFG/p.json", "policy_invalid", "name_case_differs", func(repo string) {
		commitFile(repo, "cfg/p.json", singleRootPolicyJSON)
		if _, err := os.Lstat(filepath.Join(repo, "CFG")); err != nil {
			Skip("this file system tells cfg and CFG apart, so the name does not resolve")
		}
	}},
	{"a policy committed as a symbolic link, then removed", "cfg/p.json", "policy_missing", "committed_as_link", func(repo string) {
		symlinkAt(repo, "cfg/p.json")
		gitIn(repo, "add", "cfg/p.json")
		gitInWithIdentity(repo, "commit", "-q", "-m", "a policy as a link")
		removeTree(repo, "cfg")
	}},
	{"a policy committed as a symbolic link, with a regular file on disk", "cfg/p.json", "policy_invalid", "committed_as_link", func(repo string) {
		commitLinkThenWriteFile(repo, "cfg/p.json")
	}},
	{"files committed under the name, with a regular file on disk", "cfg", "policy_invalid", "directory", func(repo string) {
		commitDirectoryThenWriteFile(repo, "cfg")
	}},
	{"a policy staged and never committed, then deleted", "cfg/p.json", "policy_missing", "staged_copy", func(repo string) {
		writeWorktreeFile(repo, "cfg/p.json", singleRootPolicyJSON)
		gitIn(repo, "add", "cfg/p.json")
		removeTree(repo, "cfg")
	}},
	{"a symbolic link staged and never committed", "cfg/p.json", "policy_missing", "staged_not_a_file", func(repo string) {
		symlinkAt(repo, "cfg/p.json")
		gitIn(repo, "add", "cfg/p.json")
		removeTree(repo, "cfg")
	}},
	{"all three sides of a conflict for a policy that was never committed", "cfg/p.json", "policy_missing", "merge_conflict", func(repo string) {
		stageThreeSidesOf(repo, "cfg/p.json")
	}},
	{"a committed policy marked skip-worktree, then deleted", "cfg/p.json", "policy_missing", "skip_worktree", func(repo string) {
		commitFile(repo, "cfg/p.json", singleRootPolicyJSON)
		gitIn(repo, "update-index", "--skip-worktree", "cfg/p.json")
		removeTree(repo, "cfg")
	}},
	{"a committed policy a sparse checkout leaves out, then deleted", "cfg/p.json", "policy_missing", "sparse_checkout", func(repo string) {
		commitFile(repo, "cfg/p.json", singleRootPolicyJSON)
		gitIn(repo, "config", "core.sparseCheckout", "true")
		gitIn(repo, "update-index", "--skip-worktree", "cfg/p.json")
		removeTree(repo, "cfg")
	}},
	{"a policy file inside a submodule checkout", "vendor/sub/coach.json", "policy_missing", "in_submodule", func(repo string) {
		addSubmodule(repo)
	}},
	{"a policy missing from inside a submodule checkout", "vendor/sub/cfg/p.json", "policy_missing", "in_submodule", func(repo string) {
		addSubmodule(repo)
	}},
	{"a path that is a submodule, with its checkout there", "vendor/sub", "policy_invalid", "in_submodule", func(repo string) {
		addSubmodule(repo)
	}},
	{"a path that is a submodule, with its checkout removed", "vendor/sub", "policy_missing", "in_submodule", func(repo string) {
		addSubmodule(repo)
		gitIn(repo, "submodule", "deinit", "-f", "vendor/sub")
		removeTree(repo, "vendor/sub")
	}},
	{"a committed policy deleted while another git process holds the index", "cfg/p.json", "policy_missing", "index_locked", func(repo string) {
		commitThenRemove(repo, "cfg/p.json")
		Expect(os.WriteFile(filepath.Join(repo, ".git", "index.lock"), nil, 0o644)).To(Succeed())
	}},
}

func requireTheCauseOfTheGap(policy, wantGap, wantCause string, arrange func(repo string)) {
	repo := commitScannableGoRepo()
	arrange(repo)

	doc := expectCheckProject(repo, "--project-config", policy)
	stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, pathWithoutNode(), "--baseline", "--check-project", "--project-language", "typescript", "--project-config", policy)

	ExpectWithOffset(1, causeOfGap(doc, wantGap)).To(Equal(wantCause), "gaps: %+v", doc.Gaps)
	ExpectWithOffset(1, exitCode).To(Equal(0), "stderr: %s", stderr)
	ExpectWithOffset(1, string(stdout)).To(ContainSubstring("  "+wantGap+" cause="+wantCause+"\n"), "the text prints the same field:\n%s", stdout)
}

// documentedCauseWords is the CLI doc, which lists the stable words a consumer may match on.
func documentedCauseWords() string {
	text, err := os.ReadFile(filepath.Join("..", "..", "docs", "cli-codesignal.md"))
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	return string(text)
}

// causeTableArgs is the body of the table followed by one entry per row.
// rawGaps are the gaps of --check-project's JSON as maps, which tell an absent key from an empty one.
func rawGaps(repo, policy string) []map[string]any {
	stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, pathWithoutNode(), "--baseline", "--check-project", "--project-language", "typescript", "--project-config", policy, "--format", "json")
	ExpectWithOffset(1, exitCode).To(Equal(0), "stderr: %s", stderr)
	var raw struct {
		Gaps []map[string]any `json:"gaps"`
	}
	ExpectWithOffset(1, json.Unmarshal(stdout, &raw)).To(Succeed(), "stdout: %s", stdout)
	return raw.Gaps
}

func causeTableArgs() []any {
	entries := []any{requireTheCauseOfTheGap}
	for _, row := range causeRows {
		entries = append(entries, Entry(row.name, row.policy, row.gap, row.cause, row.arrange))
	}
	return entries
}

// declaredPolicyCauses are the values of the PolicyCause constants, read from the source that declares them.
func declaredPolicyCauses() []string {
	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join("..", "..", "internal", "codesignalcli", "projectreadiness", "policy_cause.go"), nil, 0)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	var words []string
	ast.Inspect(file, func(node ast.Node) bool {
		if literal, ok := node.(*ast.BasicLit); ok && literal.Kind == token.STRING {
			word, unquoteErr := strconv.Unquote(literal.Value)
			ExpectWithOffset(1, unquoteErr).NotTo(HaveOccurred())
			words = append(words, word)
		}
		return true
	})
	return words
}

var _ = Describe("coach codesignal --check-project: the cause of a policy gap that offers no way out", func() {
	DescribeTable("names the cause on the gap, in JSON and in text", causeTableArgs()...)

	It("names no cause when a restore or a new policy fixes the gap", func() {
		repo := commitScannableGoRepo()
		commitThenRemove(repo, "cfg/p.json")

		doc := expectCheckProject(repo, "--project-config", "cfg/p.json")

		Expect(causeOfGap(doc, "policy_missing")).To(BeEmpty(), "a gap with a remedy needs no cause: %+v", doc.Gaps)
		Expect(nextActionKinds(doc)).To(ContainElement("restore_policy"))
		for _, gap := range rawGaps(repo, "cfg/p.json") {
			Expect(gap).NotTo(HaveKey("cause"), "the field is absent, not empty, from every gap that has none: %v", gap)
		}
	})

	// Authoring into a submodule produces a file that is then refused as uncommittable here.
	It("offers no new policy for one that is missing from inside a submodule", func() {
		repo := commitScannableGoRepo()
		addSubmodule(repo)

		doc := expectCheckProject(repo, "--project-config", "vendor/sub/cfg/p.json")
		stderr := scanExplicitPolicy(repo, "vendor/sub/cfg/p.json")

		Expect(nextActionKinds(doc)).NotTo(ContainElement("author_policy"), "next actions: %+v", doc.NextActions)
		Expect(string(stderr)).NotTo(ContainSubstring("--suggest-project-config"), "stderr:\n%s", stderr)
		Expect(string(stderr)).To(ContainSubstring(`inside the submodule "vendor/sub"`), "stderr:\n%s", stderr)
	})

	It("has a row for every cause the package declares, or says why the CLI cannot report it", func() {
		covered := make([]string, 0, len(causeRows))
		for _, row := range causeRows {
			covered = append(covered, row.cause)
		}

		declared := declaredPolicyCauses()

		for _, word := range declared {
			_, unreachable := unreachableCauses[word]
			Expect(slices.Contains(covered, word) || unreachable).To(BeTrue(), "the cause %q is declared and neither pinned by a row nor explained as unreachable", word)
		}
		for _, word := range covered {
			Expect(declared).To(ContainElement(word), "a row pins %q, which is not declared", word)
		}
		documented := documentedCauseWords()
		for _, word := range declared {
			Expect(documented).To(ContainSubstring("`"+word+"`"), "the cause %q is declared and is not in docs/cli-codesignal.md", word)
		}
	})
})
