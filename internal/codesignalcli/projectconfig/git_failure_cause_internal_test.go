package projectconfig

import (
	"errors"
	"strings"
	"testing"
)

func requireTheIndexLeftUnread(t *testing.T, status *ConfigError, why string) {
	t.Helper()
	if !status.UnreadStagedEdit {
		t.Errorf("UnreadStagedEdit = false; %s", why)
	}
}

// GitFailure words the failure for the user, and keeps git's own error behind it, so
// a caller that has to classify the failure can reach the error and not only the text.
func TestAGitFailureKeepsGitsOwnErrorReachable(t *testing.T) {
	boom := errors.New("boom: git said no")
	cases := map[string]indexFake{
		"a tree lookup that fails":    {lsTreeErr: boom},
		"an index listing that fails": {lsFilesErr: boom},
	}
	for name, fake := range cases {
		t.Run(name, func(t *testing.T) { requireGitsErrorBehindTheFailure(t, statusWhenGitFails(t, fake), boom) })
	}
}

func requireGitsErrorBehindTheFailure(t *testing.T, status *ConfigError, boom error) {
	t.Helper()
	wordsTheQuestion := strings.HasPrefix(status.GitFailure.Error(), "could not check ")
	if !errors.Is(status.GitFailure, boom) || !wordsTheQuestion {
		t.Errorf("GitFailure %q: leads to git's own error = %v, words the question that went unanswered = %v", status.GitFailure, errors.Is(status.GitFailure, boom), wordsTheQuestion)
	}
}
