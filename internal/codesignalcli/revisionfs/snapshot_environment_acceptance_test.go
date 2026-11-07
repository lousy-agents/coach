package revisionfs

import (
	"context"
	"io"
	"io/fs"
	"os/exec"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	committedSource   = "package main\n\nimport \"original\"\n"
	replacementSource = "package main\n\nimport \"replaced\"\n"
)

// commitMainWithAReplacedBlob commits main.go and then tells git, as `git replace` does, to show
// replacementSource wherever it would show the committed blob.
func commitMainWithAReplacedBlob(dir string) string {
	sha := gitfixture.CommitFile(GinkgoT(), dir, "main.go", committedSource)
	committed := gitAnswer(dir, nil, "rev-parse", sha+":main.go")
	replacement := gitAnswer(dir, strings.NewReader(replacementSource), "hash-object", "-w", "--stdin")
	gitAnswer(dir, nil, "replace", committed, replacement)
	return sha
}

func gitAnswer(dir string, stdin io.Reader, args ...string) string {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Stdin = stdin
	output, err := cmd.Output()
	Expect(err).NotTo(HaveOccurred())
	return strings.TrimSpace(string(output))
}

func snapshotContentOfMain(dir, sha string) string {
	fsys, err := New(dir, sha)
	Expect(err).NotTo(HaveOccurred())
	content, err := fs.ReadFile(fsys, "main.go")
	Expect(err).NotTo(HaveOccurred())
	return string(content)
}

func scanContentOfMain(dir, sha string) string {
	content, err := gitrepo.RunBytes(dir, "show", sha+":main.go")
	Expect(err).NotTo(HaveOccurred())
	return string(content)
}

var _ = Describe("a snapshot read of a repository whose history the caller chose to see without replacements", func() {
	var dir, sha string

	BeforeEach(func() {
		dir = gitfixture.Init(GinkgoT())
		sha = commitMainWithAReplacedBlob(dir)
	})

	It("shows the replacement while the caller has not turned replacement off", func() {
		Expect(snapshotContentOfMain(dir, sha)).To(Equal(replacementSource))
	})

	It("reads the committed file when the caller set GIT_NO_REPLACE_OBJECTS, as every other read of the scan does", func() {
		GinkgoT().Setenv("GIT_NO_REPLACE_OBJECTS", "1")

		Expect(scanContentOfMain(dir, sha)).To(Equal(committedSource))
		Expect(snapshotContentOfMain(dir, sha)).To(Equal(committedSource))
	})

	It("reads the replacement that the caller's GIT_REPLACE_REF_BASE names, as every other read of the scan does", func() {
		GinkgoT().Setenv("GIT_REPLACE_REF_BASE", "refs/not-replace/")

		Expect(scanContentOfMain(dir, sha)).To(Equal(committedSource))
		Expect(snapshotContentOfMain(dir, sha)).To(Equal(committedSource))
	})
})

var _ = Describe("the environment of a snapshot read's git child", func() {
	var dir, sha string

	BeforeEach(func() {
		dir = gitfixture.Init(GinkgoT())
		sha = gitfixture.CommitFile(GinkgoT(), dir, "main.go", committedSource)
	})

	It("is the environment every other read of the repository starts git with", func() {
		GinkgoT().Setenv("GIT_NO_REPLACE_OBJECTS", "1")
		args := []string{"show", sha + ":main.go"}

		cmd := snapshotGitCommandContext(context.Background(), dir, args...)

		Expect(cmd.Env).To(Equal(gitrepo.ReadCommandContext(context.Background(), dir, args...).Env))
	})

	DescribeTable("keeps what the caller set",
		func(name, value string) {
			GinkgoT().Setenv(name, value)

			cmd := snapshotGitCommandContext(context.Background(), dir, "show", sha+":main.go")

			Expect(cmd.Env).To(ContainElement(name + "=" + value))
		},
		Entry("so the view of the history is the caller's", "GIT_NO_REPLACE_OBJECTS", "1"),
		Entry("so the replacements are read from where the caller says", "GIT_REPLACE_REF_BASE", "refs/not-replace/"),
		Entry("so the caller's configuration is read", "GIT_CONFIG_GLOBAL", "/nonexistent/gitconfig"),
		Entry("so the caller's configuration directory is read", "XDG_CONFIG_HOME", "/nonexistent/config"),
		Entry("so the caller's system configuration is read", "GIT_CONFIG_SYSTEM", "/nonexistent/system-gitconfig"),
		Entry("so the caller's injected configuration is read", "GIT_CONFIG_COUNT", "0"),
	)

	It("keeps the settings every read of the repository adds", func() {
		cmd := snapshotGitCommandContext(context.Background(), dir, "show", sha+":main.go")

		Expect(cmd.Env).To(ContainElements("GIT_NO_LAZY_FETCH=1", "GIT_TERMINAL_PROMPT=0", "GIT_LFS_SKIP_SMUDGE=1"),
			"a blobless partial clone must never reach its remote, and git must never ask on the terminal")
	})
})
