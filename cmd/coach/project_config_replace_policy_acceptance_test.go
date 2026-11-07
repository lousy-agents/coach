package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// invalidPolicy is a document the policy schema refuses.
const invalidPolicy = `{"schema_version":"1","roots":["."],"bogus":1}` + "\n"

// bigPolicy is a valid policy document over the 1 MiB budget for a committed policy.
func bigPolicy() string {
	return `{"schema_version":"1","roots":["."],"note":"` + strings.Repeat("0123456789abcdef", 70000) + `"}` + "\n"
}

func commitGoRepoWithPolicy(policy, document string) string {
	repo := commitScannableGoRepo()
	commitFile(repo, "go.mod", "module example.com/replace\n\ngo 1.25\n")
	commitFile(repo, policy, document)
	return repo
}

func lastLine(stderr []byte) string {
	lines := strings.Split(strings.TrimSpace(string(stderr)), "\n")
	return lines[len(lines)-1]
}

// suggestCommandIn is the authoring command the Go line names for a policy that exists and cannot be read as one: the
// command that writes a candidate beside it. It is Coach's own command and creates a new file. The rest of the line is
// words.
func suggestCommandIn(line string) string {
	return regexp.MustCompile(`coach codesignal --baseline --suggest-project-config --output \S+`).FindString(line)
}

func commitStaged(repo string) {
	commit := exec.Command("git", "commit", "-q", "-m", "replace the policy")
	commit.Dir, commit.Env = repo, commitEnv
	output, err := commit.CombinedOutput()
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "git commit: %s", output)
}

// A policy that exists and is not usable as one (its document is not valid, or is over the size budget) is fixed by
// a new document at its path. Authoring creates files and refuses a target that exists, so the Go line writes the
// candidate beside the policy with Coach's own command, and says in words where it is and what to do with it. The
// authoring command is run as printed; the replacement is carried out here as the words describe it.
var _ = Describe("coach codesignal (Go): a committed policy that is invalid or over the size budget", func() {
	type document struct {
		policy string
		body   string
		args   []string
		beside string
	}
	oversized, invalid := bigPolicy(), invalidPolicy

	DescribeTable("names a candidate beside it and says in words to replace the policy with it, and the replacement is accepted",
		func(d document) {
			repo := commitGoRepoWithPolicy(d.policy, d.body)
			stdout, stderr := scanExpectingRefusal(repo, d.args...)
			line := lastLine(stderr)
			suggest := suggestCommandIn(line)
			Expect(suggest).To(HaveSuffix(" --output "+d.beside), "the candidate is written beside the policy:\n%s", stderr)
			expectNoRunnableCommand(line)
			Expect(line).To(ContainSubstring(fmt.Sprintf("writes a candidate to %q beside the policy, because authoring refuses a path that exists; compare it with %q, replace the policy with it, commit, and rerun", d.beside, d.policy)), "stderr:\n%s", stderr)

			output, exitCode := runInShell(repo, strings.Replace(suggest, "coach ", commandPath+" ", 1))
			Expect(exitCode).To(Equal(0), "the candidate command must not be refused:\ncommand: %s\noutput: %s", suggest, output)
			Expect(filepath.Join(repo, d.beside)).To(BeARegularFile())
			candidate, err := os.ReadFile(filepath.Join(repo, d.beside))
			Expect(err).NotTo(HaveOccurred())
			Expect(os.WriteFile(filepath.Join(repo, d.policy), candidate, 0o644)).To(Succeed(), "replacing the policy with the candidate, as the words say")
			Expect(os.Remove(filepath.Join(repo, d.beside))).To(Succeed())
			gitIn(repo, "add", "--", ":(literal)"+d.policy)
			commitStaged(repo)

			_, rescanStderr, rescanExit := runCoachCodesignalBaselineRaw(repo, append([]string{"--format=json"}, d.args...)...)
			Expect(rescanExit).To(Equal(0), "the replaced policy is accepted; stdout: %s stderr: %s", stdout, rescanStderr)
		},
		Entry("an over-budget document at the canonical path", document{".lousy-agents/coach.json", oversized, nil, ".lousy-agents/coach.candidate.json"}),
		Entry("an over-budget document named with --project-config", document{"cfg/p.json", oversized, []string{"--project-config", "cfg/p.json"}, "cfg/p.candidate.json"}),
		Entry("an invalid document at the canonical path", document{".lousy-agents/coach.json", invalid, nil, ".lousy-agents/coach.candidate.json"}),
		Entry("an invalid document named with --project-config", document{"cfg/p.json", invalid, []string{"--project-config", "cfg/p.json"}, "cfg/p.candidate.json"}),
	)

	It("names the next free candidate when the first is taken, so the printed command is not refused", func() {
		repo := commitGoRepoWithPolicy("cfg/p.json", bigPolicy())
		writeWorktreeFile(repo, "cfg/p.candidate.json", "taken\n")

		_, stderr := scanExpectingRefusal(repo, "--project-config", "cfg/p.json")
		suggest := suggestCommandIn(lastLine(stderr))

		Expect(suggest).To(HaveSuffix(" --output cfg/p.candidate-2.json"), "stderr:\n%s", stderr)
		Expect(lastLine(stderr)).To(ContainSubstring(`writes a candidate to "cfg/p.candidate-2.json" beside the policy`), "the words name the candidate that is free:\n%s", stderr)
		output, exitCode := runInShell(repo, strings.Replace(suggest, "coach ", commandPath+" ", 1))
		Expect(exitCode).To(Equal(0), "the candidate command must not be refused:\ncommand: %s\noutput: %s", suggest, output)
	})

	It("is described by the CLI contract in words, with no move to paste", func() {
		text := documentText("docs/cli-codesignal.md")

		Expect(text).To(ContainSubstring("the scan names the way for Go only: a candidate written beside the file with `--suggest-project-config --output`, with words that say to compare it with the file, replace the file with it, commit and rerun, and no command to paste; for TypeScript it appends no line"))
		Expect(text).NotTo(ContainSubstring("the move that puts it over the file"))
		Expect(text).To(ContainSubstring("for a TypeScript policy that exists the text of the action is to fix the policy, or move it aside and commit the move, then run guided policy authoring, as `--prepare-compiler` says in its refusal"))
	})

	It("prints no command for a name that cannot be shown", func() {
		const hostile = "cfg/p\x1b.json"
		repo := commitGoRepoWithPolicy(hostile, bigPolicy())

		_, stderr := scanExpectingRefusal(repo, "--project-config", hostile)

		Expect(string(stderr)).NotTo(ContainSubstring("\x1b"), "the escape character reached the terminal:\n%q", stderr)
		Expect(string(stderr)).NotTo(ContainSubstring("--suggest-project-config"), "a command that cannot show the path is not printed:\n%s", stderr)
	})
})

func scanExpectingRefusal(repo string, extraArgs ...string) (stdout, stderr []byte) {
	stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, append([]string{"--format=json"}, extraArgs...)...)
	ExpectWithOffset(1, exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
	return stdout, stderr
}

// For TypeScript, a scan on a terminal opens guided authoring itself, which prints a replacement and ends by asking the user
// to save it over the file. Where no terminal is attached no line is appended for a policy that exists and cannot be used,
// over the size budget as invalid: the line that named that scan would lead to a session that ends by saving over the user's
// file, and the authoring command is refused at a path that exists.
var _ = Describe("coach codesignal (TypeScript, no terminal): a committed policy over the size budget", func() {
	DescribeTable("appends no line that leads to authoring",
		func(policy string, scanArgs ...string) {
			repo := commitStubToolchainTypeScriptRepo(policy, bigPolicy())
			args := append([]string{"codesignal", "--baseline", "--project-language", "typescript", "--format=json"}, scanArgs...)

			stdout, stderr, exitCode := runCoachBinary(commandPath, repo, stubToolchainEnv(pathWithStubNode("v24.9.9")), args...)

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("guided authoring"), "the session ends by saving over the file:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("on a terminal:"), "stderr:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("--suggest-project-config"), "the authoring command is refused where the file exists:\n%s", stderr)
		},
		Entry("found at the canonical path", ".lousy-agents/coach.json"),
		Entry("named with --project-config", "pol.json", "--project-config", "pol.json"),
	)

	It("still offers guided authoring where the policy is missing, which creates the file", func() {
		repo := commitStubToolchainTypeScriptRepo("other.json", singleRootPolicyJSON)
		args := []string{"codesignal", "--baseline", "--project-language", "typescript", "--format=json", "--project-config", "pol.json"}

		stdout, stderr, exitCode := runCoachBinary(commandPath, repo, stubToolchainEnv(pathWithStubNode("v24.9.9")), args...)

		Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
		Expect(string(stderr)).To(ContainSubstring("guided authoring requires a controlling terminal"), "the create case keeps its line:\n%s", stderr)
	})
})

// regexpFor is the first match of the pattern in text, or "".
func regexpFor(pattern string, text []byte) string {
	return regexp.MustCompile(pattern).FindString(string(text))
}
