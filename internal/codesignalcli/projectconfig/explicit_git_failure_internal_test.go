package projectconfig

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
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
}

func TestATreeLookupFailureNamesTheCommitQuestionAndGitsError(t *testing.T) {
	status := statusWhenGitFails(t, indexFake{lsTreeErr: errors.New("boom: tree unreadable")})

	if !strings.Contains(status.Message, "whether it is committed") {
		t.Errorf("message %q; want it to name the question that failed, whether the policy is committed", status.Message)
	}
	if !strings.Contains(status.Message, "boom: tree unreadable") {
		t.Errorf("message %q; want it to carry git's error", status.Message)
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

func TestAGitFailureIsQuotedWithoutCoachsOwnWrapperAroundIt(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "cfg/p.json", "{}")
	blob, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD:cfg/p.json").Output()
	if err != nil {
		t.Fatal(err)
	}
	object := strings.TrimSpace(string(blob))
	if err := os.Remove(filepath.Join(repo, ".git", "objects", object[:2], object[2:])); err != nil {
		t.Fatalf("the loose object must exist in a fresh repository: %v", err)
	}
	moveAside(t, repo, "cfg/p.json")

	status, err := ExplicitOnDiskStatus(runProjectConfigGit, repo, "HEAD", "cfg/p.json")

	if err != nil || status == nil || status.GitFailure == nil {
		t.Fatalf("ExplicitOnDiskStatus = %+v, %v; want a missing-policy error that carries the git failure", status, err)
	}
	if got := strings.Count(status.Message, "coach codesignal"); got != 1 {
		t.Errorf("message %q names coach codesignal %d times; git's quoted error must not carry Coach's own wrapper", status.Message, got)
	}
	if got := strings.Count(status.GitFailure.Error(), "coach codesignal"); got != 0 {
		t.Errorf("GitFailure %q carries Coach's own wrapper around git's error", status.GitFailure)
	}
}

func TestAnIndexFailureIsQuotedWithoutCoachsOwnWrapperAroundIt(t *testing.T) {
	cases := []struct {
		name string
		fake indexFake
		want string
	}{
		{"git cannot list the index", indexFake{lsFilesErr: errors.New("exit status 128: fatal: index file smaller than expected")}, "index file smaller than expected"},
		{"git prints a line Coach cannot parse", indexFake{lsFiles: "not an index line\n"}, "cannot parse"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status := statusWhenGitFails(t, c.fake)

			if got := strings.Count(status.Message, "coach codesignal"); got != 1 {
				t.Errorf("message %q names coach codesignal %d times; the quote must not carry Coach's own wrapper", status.Message, got)
			}
			if !strings.Contains(status.Message, c.want) {
				t.Errorf("message %q; want it to carry %q", status.Message, c.want)
			}
		})
	}
}
