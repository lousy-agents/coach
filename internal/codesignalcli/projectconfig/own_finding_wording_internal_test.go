package projectconfig

import (
	"errors"
	"strings"
	"testing"
)

// "git said" belongs to what git said. When git printed something Coach cannot use, the finding is
// Coach's own, and the message words it as one: quoting it as git's would send the reader to look for
// a git error that never happened.
func TestOnlyWhatGitSaidIsWordedAsGitSaying(t *testing.T) {
	anotherPath := "H 100644 " + committedBlob + " 0\tother.json\x00"
	cases := map[string]struct {
		fake         indexFake
		wantGitSaid  bool
		wantInTheMsg string
	}{
		"git failing":                {indexFake{lsFilesErr: errors.New("boom: index unreadable")}, true, "boom: index unreadable"},
		"git listing another path":   {indexFake{lsFiles: anotherPath}, false, "git listed an entry for a path other than the one asked about"},
		"git printing a broken line": {indexFake{lsFiles: "not an index line\x00"}, false, "git printed a line this version cannot parse"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) { requireTheWording(t, c.fake, c.wantInTheMsg, c.wantGitSaid) })
	}
}

func requireTheWording(t *testing.T, fake indexFake, wantInTheMsg string, wantGitSaid bool) {
	t.Helper()
	text := statusWhenGitFails(t, fake).GitFailure.Error()
	if strings.Contains(text, "git said") != wantGitSaid || !strings.Contains(text, wantInTheMsg) {
		t.Errorf("GitFailure %q: want it to say %q, and to word it as git saying it = %v", text, wantInTheMsg, wantGitSaid)
	}
}
