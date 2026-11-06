package projectconfig

import (
	"slices"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// pathAskedAbout is the path the listing is asked about: the first pathspec after "--", as
// the :(top,literal) pathspec of the path, with the directory form's trailing slash taken off.
func pathAskedAbout(args []string) string {
	after, _ := strings.CutPrefix(args[slices.Index(args, "--")+1], ":(top,literal)")
	return strings.TrimSuffix(after, "/")
}

// entriesBelow is what `ls-files -z --full-name -- path/` prints: the paths below it, or nothing.
func (f indexFake) entriesBelow(args []string) []byte {
	if !f.holdsBelow {
		return nil
	}
	return []byte(pathAskedAbout(args) + "/inner.json\x00")
}

// revParse answers where the index lock would be, and leaves everything else about the repository,
// such as its root, to the real one on disk, since those are facts about the repository and not about
// the index.
func (f indexFake) revParse(dir string, args []string) ([]byte, error) {
	if slices.Contains(args, "--git-path") {
		// No test of the fake leaves a lock there.
		return []byte(".git/index.lock\n"), f.gitPathErr
	}
	return gitrepo.RunBytes(dir, args...)
}
