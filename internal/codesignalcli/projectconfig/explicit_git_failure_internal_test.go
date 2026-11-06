package projectconfig

import (
	"errors"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

func statusWhenGitFails(t *testing.T, fake indexFake) *ConfigError {
	t.Helper()
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "cfg/p.json", "{}")
	moveAside(t, repo, "cfg/p.json")

	status, err := ExplicitOnDiskStatus(fake.run, repo, "HEAD", "cfg/p.json")

	if err != nil || status == nil {
		t.Fatalf("ExplicitOnDiskStatus = %+v, %v; want a missing-policy error that carries the git failure", status, err)
	}
	return status
}

func TestAnIndexListingFailureNamesTheIndexQuestionAndGitsError(t *testing.T) {
	status := statusWhenGitFails(t, indexFake{lsFilesErr: errors.New("boom: index unreadable")})

	if !strings.Contains(status.Message, "what the index holds") {
		t.Errorf("message %q; want it to name the question that failed, what the index holds", status.Message)
	}
	if !strings.Contains(status.Message, "boom: index unreadable") {
		t.Errorf("message %q; want it to carry git's error", status.Message)
	}
	if strings.Contains(status.Message, "whether it is committed") {
		t.Errorf("message %q; the commit lookup succeeded, so it must not say it could not tell whether the policy is committed", status.Message)
	}
	if !status.UnreadStagedEdit {
		t.Error("UnreadStagedEdit = false; git could not list the index, so a staged edit may be there unread, and the readiness must not say the disk decided it")
	}
}

func TestATreeLookupFailureNamesTheCommitQuestionAndGitsError(t *testing.T) {
	status := statusWhenGitFails(t, indexFake{lsTreeErr: errors.New("boom: tree unreadable")})

	if !strings.Contains(status.Message, "whether it is committed") {
		t.Errorf("message %q; want it to name the question that failed, whether the policy is committed", status.Message)
	}
	if !strings.Contains(status.Message, "boom: tree unreadable") {
		t.Errorf("message %q; want it to carry git's error", status.Message)
	}
	if !status.UnreadStagedEdit {
		t.Error("UnreadStagedEdit = false; git could not say whether the policy is committed, so the index cannot be judged and a staged edit may be there unread")
	}
}

func TestAGitFailureIsSanitizedAndBounded(t *testing.T) {
	hostile := "boom\x1b[31m\n" + strings.Repeat("x", 4000)
	status := statusWhenGitFails(t, indexFake{lsFilesErr: errors.New(hostile)})

	if strings.ContainsAny(status.Message, "\x1b\n") {
		t.Errorf("message %q carries a control character from git's error", status.Message)
	}
	if len(status.Message) > 1500 {
		t.Errorf("message is %d bytes; git's error must be truncated", len(status.Message))
	}
}

// GitFailure words the failure for the user, and keeps git's own error behind it, so
// a caller that has to classify the failure can reach the error and not only the text.
func TestAGitFailureKeepsGitsOwnErrorReachable(t *testing.T) {
	boom := errors.New("boom: git said no")
	cases := []struct {
		name string
		fake indexFake
	}{
		{"a tree lookup that fails", indexFake{lsTreeErr: boom}},
		{"an index listing that fails", indexFake{lsFilesErr: boom}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status := statusWhenGitFails(t, c.fake)

			if !errors.Is(status.GitFailure, boom) {
				t.Errorf("GitFailure %v does not lead to git's own error %v", status.GitFailure, boom)
			}
			if got := status.GitFailure.Error(); !strings.HasPrefix(got, "could not check ") {
				t.Errorf("GitFailure text %q; want it to word the question that went unanswered", got)
			}
		})
	}
}
