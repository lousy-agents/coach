package projectconfig

import (
	"errors"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// Every question the restore judgment asks fails closed: when git cannot answer, no restore is
// printed. Whether the index is locked is one of them. A restore printed over a lock that could
// not be checked fails when it is run, which is the failure the lock check exists to prevent.
func TestARestoreIsNotPrintedWhenGitCannotSayWhetherTheIndexIsLocked(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "cfg/p.json", "{}")
	moveAside(t, repo, "cfg/p.json")

	status, err := ExplicitOnDiskStatus(indexFake{gitPathErr: errors.New("boom")}.run, repo, "HEAD", "cfg/p.json")

	if err != nil || status == nil {
		t.Fatalf("ExplicitOnDiskStatus = %+v, %v; want a missing-policy error", status, err)
	}
	withheld := status.Remedy == projectreadiness.RemedyNone && !strings.Contains(status.Message, "git restore")
	if !withheld || status.GitFailure == nil || status.PolicyCause != projectreadiness.CauseGitUnreadable {
		t.Errorf("remedy %d, cause %q, failure %v, message %q; want no restore, the cause git_unreadable, and the failed question named", status.Remedy, status.PolicyCause, status.GitFailure, status.Message)
	}
}
