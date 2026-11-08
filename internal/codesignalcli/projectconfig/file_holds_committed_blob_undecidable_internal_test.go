package projectconfig

import (
	"os"
	"path/filepath"
	"testing"
)

// Anything that cannot be decided counts as work in the file, because a replacement offered over a file that was not
// understood can destroy what is in it.
func TestWhatCannotBeDecidedIsNotTheCommittedBlob(t *testing.T) {
	cases := map[string]func(t *testing.T, repo string){
		"a link to a file with the committed bytes": putALinkAtThePolicyToAFileWithTheCommittedBytes,
		"a directory":         func(t *testing.T, repo string) { replaceWithDirectory(t, filepath.Join(repo, "cfg", "p.json")) },
		"a file that is gone": func(t *testing.T, repo string) { _ = os.Remove(filepath.Join(repo, "cfg", "p.json")) },
	}
	for name, arrange := range cases {
		t.Run(name, func(t *testing.T) { requireNotTheCommittedBlobAfter(t, arrange) })
	}
}

func requireNotTheCommittedBlobAfter(t *testing.T, arrange func(t *testing.T, repo string)) {
	t.Helper()
	repo, objectID := aCommittedBlob(t)
	arrange(t, repo)

	if fileHoldsTheCommittedBlob(repo, "cfg/p.json", objectID) {
		t.Errorf("what is at the policy's path was taken for the committed blob")
	}
}

func putALinkAtThePolicyToAFileWithTheCommittedBytes(t *testing.T, repo string) {
	t.Helper()
	elsewhere := filepath.Join(repo, "elsewhere.json")
	if err := os.WriteFile(elsewhere, []byte(committedBytes), 0o644); err != nil {
		t.Fatal(err)
	}
	replaceWithLink(t, filepath.Join(repo, "cfg", "p.json"), elsewhere)
}

func replaceWithLink(t *testing.T, path, target string) {
	t.Helper()
	relative, err := filepath.Rel(filepath.Dir(path), target)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(relative, path); err != nil {
		t.Skipf("no symbolic links here: %v", err)
	}
}

func replaceWithDirectory(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
}
