package gitrepo

import (
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

// configAnswers is a git whose configuration answers each of the three partial-clone questions as given: the
// promisor settings, extensions.partialclone and the partial clone filters.
type configAnswers struct {
	promisor, extension, filter []byte
	promisorErr, filterErr      error
}

func (c configAnswers) run(_ string, args ...string) ([]byte, error) {
	question := strings.Join(args, " ")
	switch {
	case strings.Contains(question, "promisor"):
		return c.promisor, c.promisorErr
	case strings.Contains(question, "extensions.partialclone"):
		return c.extension, exitStatusOne
	case strings.Contains(question, "partialclonefilter"):
		return c.filter, c.filterErr
	}
	return nil, exitStatus128
}

var _ = Describe("whether a git before 2.44 may compare staged deletions with staged additions (#737)", func() {
	DescribeTable("#737: takes the repository for a partial clone when any one marker says so, or when the configuration cannot be read",
		func(answers configAnswers, want bool) {
			Expect(mayBeAPartialClone(answers.run, "repo", false)).To(Equal(want))
		},
		Entry("a promisor remote, and no filter", configAnswers{promisor: []byte("remote.origin.promisor true\n"), filterErr: exitStatusOne}, true),
		Entry("promisor settings that cannot be read, and no filter", configAnswers{promisorErr: exitStatus128, filterErr: exitStatusOne}, true),
		Entry("filters that cannot be read, and no promisor", configAnswers{promisorErr: exitStatusOne, filterErr: exitStatus128}, true),
		Entry("no marker at all", configAnswers{promisorErr: exitStatusOne, filterErr: exitStatusOne}, false),
	)

	It("#737: takes a clone made with --filter=blob:none for a partial clone", func() {
		t := GinkgoT()
		source := gitfixture.Init(t)
		gitfixture.CommitFile(t, source, "a.go", "package a\n")
		gitfixture.Git(t, source, "config", "uploadpack.allowFilter", "true")
		clone := filepath.Join(t.TempDir(), "part")
		gitfixture.Git(t, t.TempDir(), "clone", "--quiet", "--filter=blob:none", "file://"+source, clone)

		Expect(mayBeAPartialClone(RunBytes, clone, false)).To(BeTrue())
	})
})

var _ = Describe("a probe that did not answer in time, on a git before 2.44 (#737)", func() {
	It("#737: is refused, naming why, even when git then says at once that the revision names nothing", func() {
		repository := fakeRepository{version: oldGit, probes: map[string]probeAnswer{"HEAD": {err: &BoundError{Kind: BoundTimeout, message: "git execution timed out after 2m0s"}}}}
		resolutions := 0
		run := func(dir string, args ...string) ([]byte, error) {
			if args[0] != "rev-parse" {
				return repository.run(dir, args...)
			}
			resolutions++
			if resolutions == 1 {
				return []byte(firstMissing + "\n"), nil
			}
			return nil, exitStatusOne
		}

		err := RefuseMissingObjects(run, "repo", "")

		Expect(refusalText(err)).To(ContainSubstring("could not tell whether this repository has the objects Coach reads"))
		Expect(refusalText(err)).To(ContainSubstring("timed out after 2m0s"))
	})
})
