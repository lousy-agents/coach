package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// A clone is enough to plant text in what --check-project prints: the TypeScript version a committed
// package.json declares, and the name of any uncommitted file the dirty check counts. The text output is
// read in a terminal, where an ESC sequence in either can retitle the window, clear the screen or write
// the clipboard, so a value that is not printable is shown in Go syntax. JSON needs no such treatment,
// because it escapes control characters itself.
var _ = Describe("coach codesignal --check-project: text from the repository never reaches the terminal as a control sequence", func() {
	const (
		hostileVersion = "7.0.2\x1b]0;PWNED\x07"
		hostileDirty   = "tsconfig\x1b[2J.json"
	)

	hostileRepo := func() string {
		repo := newTempGitRepo()
		commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2\u001b]0;PWNED\u0007"}}`+"\n")
		commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
		commitFile(repo, "project.json", singleRootPolicyJSON)
		Expect(os.WriteFile(filepath.Join(repo, hostileDirty), []byte("{}\n"), 0o644)).To(Succeed())
		return repo
	}
	checkProject := func(repo string, extra ...string) []byte {
		args := append([]string{"--baseline", "--check-project", "--project-language", "typescript"}, extra...)
		stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, pathWithStubNode("v24.9.9"), args...)
		ExpectWithOffset(1, exitCode).To(Equal(0), "stderr: %s", stderr)
		return stdout
	}

	It("shows the declared version and the dirty file name escaped in the text output", func() {
		stdout := checkProject(hostileRepo())

		Expect(bytes.ContainsAny(stdout, "\x1b\x07")).To(BeFalse(), "a control byte reached the terminal:\n%q", stdout)
		Expect(string(stdout)).To(ContainSubstring(`declared_version="7.0.2\x1b]0;PWNED\a"`))
		Expect(string(stdout)).To(ContainSubstring(`"tsconfig\x1b[2J.json"`))
	})

	It("leaves the JSON output as valid JSON that carries both values and no raw control byte", func() {
		stdout := checkProject(hostileRepo(), "--format", "json")

		Expect(bytes.ContainsAny(stdout, "\x1b\x07")).To(BeFalse(), "JSON must escape the control bytes itself:\n%q", stdout)
		var doc readinessResultDoc
		Expect(json.Unmarshal(stdout, &doc)).To(Succeed())
		Expect(doc.DirtyWorktree.Paths).To(ContainElement(hostileDirty))
	})

	It("prints an ordinary declared version and file name as they are", func() {
		repo := newTempGitRepo()
		commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"^5.6.3 || ~5.5"}}`+"\n")
		commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
		commitFile(repo, "project.json", singleRootPolicyJSON)
		Expect(os.WriteFile(filepath.Join(repo, "tsconfig.base.json"), []byte("{}\n"), 0o644)).To(Succeed())

		stdout := string(checkProject(repo))

		Expect(stdout).To(ContainSubstring("declared_version=^5.6.3 || ~5.5"))
		Expect(stdout).To(ContainSubstring("\n  tsconfig.base.json\n"))
	})
})
