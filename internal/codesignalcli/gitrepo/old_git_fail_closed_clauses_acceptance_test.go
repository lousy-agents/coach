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

// timedOutAt is a git question that ran out of time.
var timedOutAt = &BoundError{Kind: BoundTimeout, message: "git execution timed out after 2m0s"}

// everyResolutionTimesOut is the fake's git, except that every rev-parse runs out of time.
func everyResolutionTimesOut(fallback Runner) Runner {
	return func(dir string, args ...string) ([]byte, error) {
		if args[0] == "rev-parse" {
			return nil, timedOutAt
		}
		return fallback(dir, args...)
	}
}

var _ = Describe("a guard question that did not answer in time (#737)", func() {
	DescribeTable("#737: is refused, naming why, at every place the guard asks one",
		func(base string, repository fakeRepository, wrap func(Runner) Runner) {
			err := RefuseMissingObjects(wrap(repository.run), "repo", base)

			Expect(refusalText(err)).To(ContainSubstring("could not tell whether"), "%v", err)
			Expect(refusalText(err)).To(ContainSubstring("timed out after"), "%v", err)
		},
		Entry("a git before 2.44 resolving HEAD", "",
			fakeRepository{version: oldGit}, everyResolutionTimesOut),
		Entry("a git before 2.44 looking for the commit --base names", "main",
			fakeRepository{version: oldGit, probes: map[string]probeAnswer{firstMissing: {err: timedOutAt}}}, func(run Runner) Runner { return run }),
		Entry("a git before 2.44 probing HEAD, which git then says at once names nothing", "",
			fakeRepository{version: oldGit, probes: map[string]probeAnswer{"HEAD": {err: timedOutAt}}}, func(run Runner) Runner { return headResolvesThen(exitStatusOne, run) }),
		Entry("a git before 2.44 probing the merge base, which git then says at once names nothing", "main",
			fakeRepository{version: oldGit, mergeBase: mergeBaseSHA, probes: map[string]probeAnswer{mergeBaseSHA: {err: timedOutAt}}}, func(run Runner) Runner { return headResolvesThen(exitStatusOne, run) }),
		Entry("a current git probing a partial clone's merge base", "main",
			fakeRepository{version: currentGit, promisors: promisorOrigin, mergeBase: mergeBaseSHA, probes: map[string]probeAnswer{mergeBaseSHA: {err: timedOutAt}}}, func(run Runner) Runner { return run }),
	)
})
