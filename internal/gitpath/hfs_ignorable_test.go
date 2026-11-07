package gitpath

import "testing"

// HFS+ drops the code points in git's list (next_hfs_char) before it compares a name, and compares without
// regard to case, so a name that spells .git around them is the git directory there. Every code point of the set
// is named, with its neighbours, because a range that is one short at either end is invisible to a name that
// uses its middle.
func TestHFSIgnorableCodePointsAreExactlyTheSetGitDrops(t *testing.T) {
	ignored := []rune{0x200c, 0x200d, 0x200e, 0x200f, 0x202a, 0x202b, 0x202c, 0x202d, 0x202e, 0x206a, 0x206b, 0x206c, 0x206d, 0x206e, 0x206f, 0xfeff}
	for _, r := range ignored {
		if name := ".g" + string(r) + "it"; !NamesGitDirectory(name) {
			t.Errorf("NamesGitDirectory(%q) = false; HFS+ ignores U+%04X, so git refuses this name", name, r)
		}
	}
	notIgnored := []rune{0x200b, 0x2010, 0x2029, 0x202f, 0x2069, 0x2070, 0xfefe}
	for _, r := range notIgnored {
		if name := ".g" + string(r) + "it"; NamesGitDirectory(name) {
			t.Errorf("NamesGitDirectory(%q) = true; U+%04X is not ignored by HFS+, so this is another name", name, r)
		}
	}
}

func TestTheHFSSpellingIsMatchedInAnyCase(t *testing.T) {
	for _, name := range []string{".GI\u200cT", "\u200c.GIT", ".Gi\u206aT", ".\u206aGIT", ".G\u202eIt"} {
		if !NamesGitDirectory(name) {
			t.Errorf("NamesGitDirectory(%q) = false; git folds case before it compares with .git", name)
		}
	}
}
