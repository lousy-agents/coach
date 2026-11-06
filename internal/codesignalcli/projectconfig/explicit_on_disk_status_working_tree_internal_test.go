package projectconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

const workingTreePolicy = "cfg/p.json"

func goneWithARestoreOffered(t *testing.T, repo string) {
	t.Helper()
	moveAside(t, repo, workingTreePolicy)
}

func goneWithAStagedEdit(t *testing.T, repo string) {
	t.Helper()
	writeFile(t, repo, workingTreePolicy, `{"edited":true}`)
	runGit(t, repo, "add", workingTreePolicy)
	moveAside(t, repo, workingTreePolicy)
}

func goneAndNeverCommitted(t *testing.T, repo string) {
	t.Helper()
	moveAside(t, repo, workingTreePolicy)
	runGit(t, repo, "rm", "-q", "--cached", workingTreePolicy)
	runGit(t, repo, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "drop the policy")
}

func replacedByADirectory(t *testing.T, repo string) {
	t.Helper()
	moveAside(t, repo, workingTreePolicy)
	if err := os.Mkdir(filepath.Join(repo, workingTreePolicy), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
}

// An error that came from the working tree says so, so a caller that holds it does not read the
// working tree for the same policy again. ExplicitOnDiskStatus is the one producer of that claim, and
// it returns three differently built errors: one with a restore offered, one with none, and one for an
// entry on disk that is no usable file. Each is checked as ExplicitOnDiskStatus returns it, and not as
// a test could build it by hand.
func TestExplicitOnDiskStatusSaysTheWorkingTreeProducedEveryError(t *testing.T) {
	cases := map[string]func(t *testing.T, repo string){
		"a committed policy gone from disk, with a restore offered":               goneWithARestoreOffered,
		"a committed policy gone from disk, with a staged edit and so no restore": goneWithAStagedEdit,
		"a policy nothing commits, gone from disk":                                goneAndNeverCommitted,
		"a directory where the policy file should be":                             replacedByADirectory,
	}
	for name, arrange := range cases {
		t.Run(name, func(t *testing.T) { assertWorkingTreeProducedTheError(t, arrange) })
	}
}

func assertWorkingTreeProducedTheError(t *testing.T, arrange func(t *testing.T, repo string)) {
	t.Helper()
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, workingTreePolicy, "{}")
	arrange(t, repo)

	status, err := ExplicitOnDiskStatus(gitrepo.RunBytes, repo, "HEAD", workingTreePolicy)

	if err != nil || status == nil {
		t.Fatalf("ExplicitOnDiskStatus = %+v, %v; want an error the working tree produced", status, err)
	}
	if !status.FromWorkingTree {
		t.Errorf("FromWorkingTree is false on %q; want true, because the working tree produced this error", status.Message)
	}
}

func TestExplicitOnDiskStatusIsNilForAPolicyThatIsARegularFile(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, workingTreePolicy, "{}")

	status, err := ExplicitOnDiskStatus(gitrepo.RunBytes, repo, "HEAD", workingTreePolicy)

	if err != nil || status != nil {
		t.Errorf("ExplicitOnDiskStatus = %+v, %v; want nil, nil", status, err)
	}
}
