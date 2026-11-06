package projectconfig

import "testing"

func TestACaseMismatchPassesNoPathThatCannotBeShownOrRunWithoutQuoting(t *testing.T) {
	cases := []struct {
		name      string
		corrected string
		want      string
		avoid     string
	}{
		{"a space is quoted", "cfg/a b.json", `pass --project-config 'cfg/a b.json'`, ""},
		{"an apostrophe is quoted", "cfg/it's.json", `pass --project-config 'cfg/it'\''s.json'`, ""},
		{"a control byte is not shown", "cfg/esc\x1b[31m.json", "", "pass --project-config"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mismatch := &caseMismatchError{repoPath: "cfg/X.json", directory: "cfg", typed: "X.json", onDisk: "x.json", corrected: c.corrected}

			assertCaseMismatchAdvice(t, mismatch.config(fakeRevision{"": {"cfg"}, "cfg": {"x.json"}}.entries), splitNonEmpty(c.want), splitNonEmpty(c.avoid))
		})
	}
}

func splitNonEmpty(text string) []string {
	if text == "" {
		return nil
	}
	return []string{text}
}
