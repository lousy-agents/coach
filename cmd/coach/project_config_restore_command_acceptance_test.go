package main

import (
	"path/filepath"
	"strings"
	"unicode/utf8"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal: the restore command printed for a missing committed policy is safe to paste, whatever the name", func() {
	pasteable := []string{`q"uote.json`, "`touch PWN`.json", "a;touch PWN;b.json", "-x.json", "*.json", ":(top)x.json", "!bang#hash.json", "HEAD~1.json", "it's.json", "my policy.json", "$(touch PWN).json"}

	DescribeTable("a Go scan, run through sh",
		func(name string) {
			repo := commitScannableGoRepo()
			commitThenDelete(repo, name)

			command := restoreCommandFrom([]byte(refusedScan(repo, name)))
			Expect(command).NotTo(BeEmpty(), "the error offers no command")
			output, exitCode := runInShell(repo, command)

			Expect(exitCode).To(Equal(0), "command=%s output=%s", command, output)
			Expect(filepath.Join(repo, "PWN")).NotTo(BeAnExistingFile(), "the shell ran part of the file name")
			Expect(filepath.Join(repo, name)).To(BeARegularFile())
			Expect(gitIn(repo, "status", "--porcelain")).To(BeEmpty(), "the restore changed more than the policy")
		},
		func(name string) string { return "the name " + name },
		Entry(nil, pasteable[0]), Entry(nil, pasteable[1]), Entry(nil, pasteable[2]), Entry(nil, pasteable[3]), Entry(nil, pasteable[4]),
		Entry(nil, pasteable[5]), Entry(nil, pasteable[6]), Entry(nil, pasteable[7]), Entry(nil, pasteable[8]), Entry(nil, pasteable[9]), Entry(nil, pasteable[10]),
	)

	DescribeTable("a TypeScript scan, run through sh",
		func(name string) {
			repo := commitScannableGoRepo()
			commitThenDelete(repo, name)

			command := restoreCommandFrom([]byte(refusedScan(repo, name, "--project-language", "typescript")))
			Expect(command).NotTo(BeEmpty(), "the error offers no command")
			output, exitCode := runInShell(repo, command)

			Expect(exitCode).To(Equal(0), "command=%s output=%s", command, output)
			Expect(filepath.Join(repo, "PWN")).NotTo(BeAnExistingFile(), "the shell ran part of the file name")
			Expect(filepath.Join(repo, name)).To(BeARegularFile())
		},
		func(name string) string { return "the name " + name },
		Entry(nil, pasteable[1]), Entry(nil, pasteable[2]), Entry(nil, pasteable[3]), Entry(nil, pasteable[5]), Entry(nil, pasteable[8]), Entry(nil, pasteable[9]), Entry(nil, pasteable[10]),
	)

	DescribeTable("a name with a control, format or separator character gets no command and none of it is printed raw",
		func(name string) {
			repo := commitScannableGoRepo()
			commitThenDelete(repo, name)

			for _, language := range []string{"go", "typescript"} {
				stderr := refusedScan(repo, name, "--project-language", language)

				Expect(stderr).NotTo(ContainSubstring("git restore"), language)
				Expect(stderr).To(ContainSubstring("not printed here"), language)
				for _, r := range stderr {
					Expect(r == '\n' || r == '\t' || (r >= 0x20 && r != 0x7f && r != 0x202e && r != 0x2028 && r != 0x200b && utf8.ValidRune(r))).To(BeTrue(), "raw %U reached the terminal (%s)", r, language)
				}
			}
		},
		func(name string) string {
			return "the name " + strings.NewReplacer("\x07", `\a`, "\n", `\n`, "\x7f", `\x7f`).Replace(name)
		},
		Entry(nil, "bell\x07.json"), Entry(nil, "nl\nname.json"), Entry(nil, "x\u202eY.json"), Entry(nil, "x\u2028y.json"), Entry(nil, "x\u200by.json"), Entry(nil, "del\x7f.json"),
	)

	It("--check-project prints no command for any of these names, only a restore_policy next action that is not executable", func() {
		for _, name := range []string{"$(touch PWN).json", "nl\nname.json", "a;touch PWN;b.json"} {
			repo := commitScannableGoRepo()
			commitThenDelete(repo, name)

			stdout, stderr, exitCode := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--check-project", "--project-language", "typescript", "--format", "json", "--project-config", name)

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(string(stdout)).To(ContainSubstring(`"kind":"restore_policy","executable":false`))
			Expect(string(stdout) + string(stderr)).NotTo(ContainSubstring("git restore"))
			Expect(filepath.Join(repo, "PWN")).NotTo(BeAnExistingFile())
		}
	})
})
