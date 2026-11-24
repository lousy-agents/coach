package projectconfig

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

const workingTreePolicy = "cfg/p.json"

func TestExplicitOnDiskStatusIsNilForAPolicyThatIsARegularFile(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, workingTreePolicy, validPolicy)

	status, err := ExplicitOnDiskStatus(gitrepo.RunBytes, repo, "HEAD", workingTreePolicy)

	if err != nil || status != nil {
		t.Errorf("ExplicitOnDiskStatus = %+v, %v; want nil, nil", status, err)
	}
}
