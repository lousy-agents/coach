package gitrepo

import (
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// StatusWarnings is what `git status` said on stderr while it still succeeded.
// The scan treats any warning as part of the tree it may not have read.
type StatusWarnings struct {
	// UnreadableDirectories names, once each, the directories git could not
	// open. A name is stored without git's trailing slash and printable-escaped:
	// one with a control character or bytes that are not UTF-8 is quoted, and
	// one that cannot be extracted from the line is "a directory whose name
	// cannot be shown".
	UnreadableDirectories []string
	// Unrecognized counts the warnings this parser does not know, repeats
	// included, which could be about anything the scan did not read.
	Unrecognized int
}

// None is true when git warned about nothing.
func (w StatusWarnings) None() bool {
	return len(w.UnreadableDirectories) == 0 && w.Unrecognized == 0
}

const (
	unreadableDirectoryPrefix = "warning: could not open directory '"
	unnameableDirectory       = "a directory whose name cannot be shown"
)

// ParseStatusWarnings reads git's stderr, in the C locale the scan runs git in,
// failing closed: a line is recognized by its prefix, the name is taken up to
// the last quote-colon-space on the line, and a line that is a warning but not
// one of these is reported as unrecognized instead of dropped.
func ParseStatusWarnings(stderr []byte) StatusWarnings {
	var warnings StatusWarnings
	for _, line := range strings.Split(string(stderr), "\n") {
		switch {
		case strings.HasPrefix(line, unreadableDirectoryPrefix):
			warnings.UnreadableDirectories = addDirectory(warnings.UnreadableDirectories, line)
		case strings.HasPrefix(line, "warning:"):
			warnings.Unrecognized++
		}
	}
	return warnings
}

func addDirectory(directories []string, line string) []string {
	rest := strings.TrimPrefix(line, unreadableDirectoryPrefix)
	name := unnameableDirectory
	if end := strings.LastIndex(rest, "': "); end >= 0 {
		name = printable(strings.TrimSuffix(rest[:end], "/"))
	}
	if slices.Contains(directories, name) {
		return directories
	}
	return append(directories, name)
}

func printable(name string) string {
	if !utf8.ValidString(name) || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return strconv.Quote(name)
	}
	return name
}
