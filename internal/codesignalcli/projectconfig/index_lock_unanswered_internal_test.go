package projectconfig

import (
	"errors"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
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
	notReportedAsLocked := !strings.Contains(status.Message, "The index is locked, so a restore would fail")
	withheld := notReportedAsLocked && status.Remedy == projectreadiness.RemedyNone && !strings.Contains(status.Message, "git restore")
	if !withheld || status.GitFailure == nil || status.PolicyCause != projectreadiness.PolicyCause("git_unreadable") {
		t.Errorf("remedy %d, cause %q, failure %v, message %q; want no restore, the cause git_unreadable, and the failed question named", status.Remedy, status.PolicyCause, status.GitFailure, status.Message)
	}
}

// Each outcome of the lock question is explicit and tested, in both directions: an error is never "not
// locked" (the restore would be printed over a lock that was not checked), and a failing rev-parse is never
// "locked" (the user would be told to wait for a git process that is not there).
func TestTheLockQuestionHasOneExplicitOutcomePerAnswer(t *testing.T) {
	cases := map[string]struct {
		arrange    func(t *testing.T, repo string) gitrepo.Runner
		wantLocked bool
		wantErr    bool
	}{
		"the lock file is there": {func(t *testing.T, repo string) gitrepo.Runner {
			lockTheIndex(t, repo)
			return gitrepo.RunBytes
		}, true, false},
		"there is no lock file": {func(t *testing.T, repo string) gitrepo.Runner { return gitrepo.RunBytes }, false, false},
		"git fails to say where the lock would be": {func(t *testing.T, repo string) gitrepo.Runner {
			return failingEach(map[string]error{"rev-parse --git-path": errors.New("boom")})
		}, false, true},
		"git prints no path for the lock": {func(t *testing.T, repo string) gitrepo.Runner {
			return func(string, ...string) ([]byte, error) { return []byte("\n"), nil }
		}, false, true},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) { requireTheLockOutcome(t, c.arrange, c.wantLocked, c.wantErr) })
	}
}

func requireTheLockOutcome(t *testing.T, arrange func(t *testing.T, repo string) gitrepo.Runner, wantLocked, wantErr bool) {
	t.Helper()
	repo := gitfixture.Init(t)
	run := arrange(t, repo)

	locked, err := indexIsLocked(run, repo)

	if locked != wantLocked || (err != nil) != wantErr {
		t.Errorf("indexIsLocked = %v, %v; want locked %v and an error = %v", locked, err, wantLocked, wantErr)
	}
}
