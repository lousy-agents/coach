package projectconfig

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

// gitSaysUnset is what git leaves when `git config --get` finds no value for a key: a real exit 1 with
// nothing on stderr.
var gitSaysUnset = func() error {
	err := exec.Command("sh", "-c", "exit 1").Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		panic("sh -c 'exit 1' did not fail with an exit status")
	}
	return err
}()

// Whether a sparse checkout is on decides which advice a skip-worktree entry gets: add the directory to the
// checkout, or clear the mark. An unset key is a plain no; a question git cannot answer is not, because the
// advice for a sparse checkout is wrong for a mark set by hand and the other way round.
func TestAFailedSparseCheckoutQuestionIsNotTakenForOff(t *testing.T) {
	entry := skippedIndexLine("100644", committedBlob, "0")
	failed := indexFake{lsFiles: entry, configErr: errors.New("exit status 128: fatal: bad config line 1 in file .git/config")}

	got := judgeRestore(failed.run, "repo", "HEAD", "pol.json")

	if got.verdict != restoreUndecidable || got.failure == nil || !strings.Contains(got.failure.Error(), "whether a sparse checkout is on") {
		t.Errorf("judgeRestore = verdict %d, failure %v; want an undecidable verdict that names the sparse checkout question", got.verdict, got.failure)
	}
}

func TestAnUnsetSparseCheckoutKeyIsOffAndASetOneIsOn(t *testing.T) {
	entry := skippedIndexLine("100644", committedBlob, "0")

	unset := judgeRestore(indexFake{lsFiles: entry}.run, "repo", "HEAD", "pol.json")
	set := judgeRestore(indexFake{lsFiles: entry, sparse: true}.run, "repo", "HEAD", "pol.json")

	if unset.verdict != restoreSkipWorktreeMarked || set.verdict != restoreSkippedBySparseCheckout {
		t.Errorf("unset key: verdict %d, set key: verdict %d; want the skip-worktree mark (%d) and the sparse checkout (%d)", unset.verdict, set.verdict, restoreSkipWorktreeMarked, restoreSkippedBySparseCheckout)
	}
}
