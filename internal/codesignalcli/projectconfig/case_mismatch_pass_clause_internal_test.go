package projectconfig

import (
	"strings"
	"testing"
)

// The path the advice passes is the one the revision holds, quoted for a shell, and nothing when it cannot be shown.
func TestACaseMismatchPassesNoPathThatCannotBeShownOrRunWithoutQuoting(t *testing.T) {
	cases := []struct {
		name  string
		held  string // the file name the revision holds
		want  string
		avoid string
	}{
		{"a space is quoted", "a b.json", `pass --project-config 'cfg/a b.json'`, ""},
		{"an apostrophe is quoted", "it's.json", `pass --project-config 'cfg/it'\''s.json'`, ""},
		{"a control byte is not shown", "esc\x1b[31m.json", "", "pass --project-config"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { requireTheQuotedPass(t, c.held, c.want, c.avoid) })
	}
}

func requireTheQuotedPass(t *testing.T, held, want, avoid string) {
	t.Helper()
	typed := strings.ToUpper(held)
	mismatch := newCaseMismatch("cfg/"+typed, []string{"cfg", typed}, 1, held)

	assertCaseMismatchAdvice(t, mismatch.config(fakeRevision{"": {"cfg"}, "cfg": {held}}.entries), splitNonEmpty(want), splitNonEmpty(avoid))
}

func splitNonEmpty(text string) []string {
	if text == "" {
		return nil
	}
	return []string{text}
}
