package gitrepo

import (
	"fmt"
	"strings"
)

// NamesGitDirectory reports whether a path component is the name of a repository's git directory, in any
// spelling git protects against when it checks a tree out: ".git" in any case; with trailing dots and spaces,
// or followed by ":" or "\", as NTFS reads it; "git~1", the short name NTFS gives the directory; and with
// the code points HFS+ ignores in a name. It follows git's is_ntfs_dotgit, is_hfs_dotgit and verify_dotfile,
// which refuse such a path whatever the platform, so a tree that holds one is not a tree a checkout can write.
func NamesGitDirectory(component string) bool {
	return ntfsNamesGitDirectory(component) || hfsNamesGitDirectory(component)
}

func ntfsNamesGitDirectory(component string) bool {
	rest, named := cutFoldedPrefix(component, ".git")
	if !named {
		rest, named = cutFoldedPrefix(component, "git~1")
	}
	if !named {
		return false
	}
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case '\\', ':':
			return true
		case '.', ' ':
		default:
			return false
		}
	}
	return true
}

// hfsNamesGitDirectory is ntfsNamesGitDirectory's counterpart for HFS+, which drops the code points it
// ignores before it compares names.
func hfsNamesGitDirectory(component string) bool {
	return strings.EqualFold(strings.Map(dropHFSIgnorable, component), ".git")
}

func dropHFSIgnorable(r rune) rune {
	switch {
	case r >= 0x200c && r <= 0x200f, r >= 0x202a && r <= 0x202e, r >= 0x206a && r <= 0x206f, r == 0xfeff:
		return -1
	}
	return r
}

// cutFoldedPrefix is strings.CutPrefix for an ASCII prefix that ignores case.
func cutFoldedPrefix(s, prefix string) (string, bool) {
	if len(s) < len(prefix) || !strings.EqualFold(s[:len(prefix)], prefix) {
		return "", false
	}
	return s[len(prefix):], true
}

// refuseUncheckablePath is the error for a path of a tree that git would not write to a file system: one with
// a component that names a git directory, or that is empty, "." or "..". A snapshot that wrote such a path
// would hold a directory git and Go's version control stamping take for a repository, whose config names
// programs git runs.
func refuseUncheckablePath(path string) error {
	for _, component := range strings.Split(path, "/") {
		switch {
		case component == "" || component == "." || component == "..":
			return fmt.Errorf("the tree holds the path %q, which has a component that is not a name", path)
		case NamesGitDirectory(component):
			return fmt.Errorf("the tree holds the path %q, which is inside a git directory: git refuses to check out such a path, and a git directory in a source snapshot is one whose config names programs git runs", path)
		}
	}
	return nil
}
