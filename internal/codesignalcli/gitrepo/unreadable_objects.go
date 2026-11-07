package gitrepo

import (
	"slices"
	"strings"
)

// maxSaidRunes bounds what git said about a damaged object in a message: its errors are a line or two, and a
// store with a great many damaged objects would print one for each.
const maxSaidRunes = 300

// separateUnreadable moves from missing to unreadable the objects that git named in an error line on stderr.
// `git cat-file --batch` answers "<id> missing" for an object that is not in the store and for one that is and
// cannot be read, a loose object that does not inflate or a pack that does not match its index; the second is
// told apart only by the error git printed beside it, which is read once the stream has ended. An object that
// no error names stays missing, and a warning is not an error: a partial clone that cannot fetch says so in one.
func (o *objectOutcome) separateUnreadable(stderr string) {
	var lines []string
	for _, line := range strings.Split(stderr, "\n") {
		if strings.HasPrefix(line, "error: ") || strings.HasPrefix(line, "fatal: ") {
			lines = append(lines, strings.TrimSpace(line))
		}
	}
	if len(lines) == 0 {
		return
	}
	var stillMissing []string
	for _, oid := range o.missing {
		if slices.ContainsFunc(lines, func(line string) bool { return strings.Contains(line, oid) }) {
			o.unreadable = append(o.unreadable, oid)
			continue
		}
		stillMissing = append(stillMissing, oid)
	}
	if len(o.unreadable) == 0 {
		return
	}
	o.missing = stillMissing
	o.said = saidAbout(lines)
}

func saidAbout(lines []string) string {
	said := []rune(TerminalSafe(strings.Join(lines, "; ")))
	if len(said) > maxSaidRunes {
		said = append(said[:maxSaidRunes], '…')
	}
	return string(said)
}
