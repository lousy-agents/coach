package projectconfig

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

// A git question that fails is an operational error only where an offer depends on its answer.
// For a path the working tree already found unusable no offer is made, so the failure leaves the
// verdict as the scan gives it and says only that the index could not be ruled out.
func TestAGitFailureAboutAPathTheWorkingTreeRefusedIsNotAnOperationalFailure(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "cfg/p.json", "{}")
	moveAside(t, repo, "cfg/p.json")
	if err := os.MkdirAll(filepath.Join(repo, "cfg", "p.json"), 0o755); err != nil {
		t.Fatal(err)
	}

	status, err := ExplicitOnDiskStatus(indexFake{lsFilesErr: errors.New("boom")}.run, repo, "HEAD", "cfg/p.json")

	if err != nil || status == nil || status.Kind != KindInvalid {
		t.Fatalf("ExplicitOnDiskStatus = %+v, %v; want the refusal of a directory where a file should be", status, err)
	}
	if status.GitFailure != nil || !status.UnreadStagedEdit {
		t.Errorf("GitFailure = %v, UnreadStagedEdit = %v; want no operational failure and the index left unread", status.GitFailure, status.UnreadStagedEdit)
	}
}
