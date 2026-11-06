package projectconfig

import (
	"slices"
	"strings"
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
