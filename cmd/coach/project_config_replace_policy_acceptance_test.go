package main

import (
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

// replaceCommandsIn are the two commands the Go line names for a policy that exists and cannot be read as one: the
// authoring command that writes a candidate beside it, and the move that puts the candidate over the policy.
func replaceCommandsIn(line string) (suggest, move string) {
	suggest = regexp.MustCompile(`coach codesignal --baseline --suggest-project-config --output \S+`).FindString(line)
	move = regexp.MustCompile(`top="\$\(git rev-parse --show-toplevel\)" && mv -f -- .* && git -C "\$top" add -- \S+`).FindString(line)
	return suggest, move
}

func commitStaged(repo string) {
	commit := exec.Command("git", "commit", "-q", "-m", "replace the policy")
	commit.Dir, commit.Env = repo, commitEnv
	output, err := commit.CombinedOutput()
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "git commit: %s", output)
}

// A policy that exists and is not usable as one (its document is not valid, or is over the size budget) is fixed by
// a new document at its path. Authoring creates files and refuses a target that exists, so the Go line writes the
// candidate beside the policy, and names the move that puts it over the file. Every command is run as printed.
var _ = Describe("coach codesignal (Go): a committed policy that is invalid or over the size budget", func() {
	type document struct {
		policy string
		body   string
		args   []string
		beside string
	}
	oversized, invalid := bigPolicy(), invalidPolicy

	DescribeTable("names a candidate beside it and the move that replaces it, and the replacement is accepted",
		func(d document) {
			repo := commitGoRepoWithPolicy(d.policy, d.body)
			stdout, stderr := scanExpectingRefusal(repo, d.args...)
			suggest, move := replaceCommandsIn(lastLine(stderr))
			Expect(suggest).To(HaveSuffix(" --output "+d.beside), "the candidate is written beside the policy:\n%s", stderr)
			Expect(move).NotTo(BeEmpty(), "the move that replaces the policy is named:\n%s", stderr)

			output, exitCode := runInShell(repo, strings.Replace(suggest, "coach ", commandPath+" ", 1))
			Expect(exitCode).To(Equal(0), "the candidate command must not be refused:\ncommand: %s\noutput: %s", suggest, output)
			Expect(filepath.Join(repo, d.beside)).To(BeARegularFile())
			subdirectory := filepath.Join(repo, "docs", "deeper")
			Expect(os.MkdirAll(subdirectory, 0o755)).To(Succeed())
			moved, movedExit := runInShell(subdirectory, move)
			Expect(movedExit).To(Equal(0), "the move must work from a subdirectory:\ncommand: %s\noutput: %s", move, moved)
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
		suggest, _ := replaceCommandsIn(lastLine(stderr))

		Expect(suggest).To(HaveSuffix(" --output cfg/p.candidate-2.json"), "stderr:\n%s", stderr)
		output, exitCode := runInShell(repo, strings.Replace(suggest, "coach ", commandPath+" ", 1))
		Expect(exitCode).To(Equal(0), "the candidate command must not be refused:\ncommand: %s\noutput: %s", suggest, output)
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

// For TypeScript the replace path is the scan itself: on a terminal it opens guided authoring and prints a
// replacement to save over the file. Where no terminal is attached the line names that scan, for an over-budget
// document as for an invalid one, and not the authoring command, which would be refused at a path that exists.
var _ = Describe("coach codesignal (TypeScript, no terminal): a committed policy over the size budget", func() {
	DescribeTable("names the scan that can replace it and not the authoring command",
		func(policy string, scanArgs ...string) {
			repo := commitStubToolchainTypeScriptRepo(policy, bigPolicy())
			args := append([]string{"codesignal", "--baseline", "--project-language", "typescript", "--format=json"}, scanArgs...)

			stdout, stderr, exitCode := runCoachBinary(commandPath, repo, stubToolchainEnv(pathWithStubNode("v24.9.9")), args...)

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			line := lastLine(stderr)
			Expect(line).To(ContainSubstring("guided authoring opens there and prints a replacement"), "stderr:\n%s", stderr)
			Expect(line).NotTo(ContainSubstring("--suggest-project-config"), "the authoring command is refused where the file exists:\n%s", stderr)
		},
		Entry("found at the canonical path", ".lousy-agents/coach.json"),
		Entry("named with --project-config", "pol.json", "--project-config", "pol.json"),
	)
})
