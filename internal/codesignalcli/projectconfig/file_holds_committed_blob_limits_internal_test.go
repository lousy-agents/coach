package projectconfig

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAFileTooLargeToCompareIsNotTheCommittedBlob(t *testing.T) {
	repo, objectID := aCommittedBlob(t)
	file, err := os.OpenFile(filepath.Join(repo, "cfg", "p.json"), os.O_WRONLY|os.O_TRUNC, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	if err := file.Truncate(maxComparedBytes + 1); err != nil {
		t.Skipf("no sparse file of that size here: %v", err)
	}

	if fileHoldsTheCommittedBlob(repo, "cfg/p.json", objectID) {
		t.Errorf("a file past the bound was taken for the committed blob")
	}
}

// aSha256Policy is a policy written in a repository that names objects with SHA-256, and the id git gives it.
func aSha256Policy(t *testing.T) (repo, path, objectID string) {
	t.Helper()
	repo = t.TempDir()
	if err := exec.Command("git", "-C", repo, "init", "-q", "--object-format=sha256").Run(); err != nil {
		t.Skipf("this git cannot make a sha256 repository: %v", err)
	}
	path = filepath.Join(repo, "p.json")
	if err := os.WriteFile(path, []byte(committedBytes), 0o644); err != nil {
		t.Fatal(err)
	}
	output, err := exec.Command("git", "-C", repo, "hash-object", "--no-filters", "p.json").Output()
	if err != nil {
		t.Fatal(err)
	}
	return repo, path, strings.TrimSpace(string(output))
}

// A repository that names objects with SHA-256 gets the same answer.
func TestARepositoryThatNamesObjectsWithSha256GetsTheSameAnswer(t *testing.T) {
	repo, path, objectID := aSha256Policy(t)

	if len(objectID) != 64 || !fileHoldsTheCommittedBlob(repo, "p.json", objectID) {
		t.Errorf("the file was not the blob %q of a sha256 repository", objectID)
	}
	if err := os.WriteFile(path, []byte(committedBytes+" "), 0o644); err != nil {
		t.Fatal(err)
	}
	if fileHoldsTheCommittedBlob(repo, "p.json", objectID) {
		t.Errorf("an edited file was the blob of a sha256 repository")
	}
}
