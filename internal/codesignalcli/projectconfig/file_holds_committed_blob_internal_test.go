package projectconfig

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

const committedBytes = "{\"schema_version\":\"1\",\"roots\":[\".\"]}\n"

// aCommittedBlob is a repository with a policy committed, and the object id git gives it.
func aCommittedBlob(t *testing.T) (repo, objectID string) {
	t.Helper()
	repo = gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "cfg/p.json", committedBytes)
	output, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD:cfg/p.json").Output()
	if err != nil {
		t.Fatal(err)
	}
	return repo, strings.TrimSpace(string(output))
}

func TestAFileWithTheCommittedBytesHoldsTheCommittedBlob(t *testing.T) {
	repo, objectID := aCommittedBlob(t)

	if !fileHoldsTheCommittedBlob(repo, "cfg/p.json", objectID) {
		t.Errorf("the file as committed is not the committed blob %s", objectID)
	}
}

func TestAFileThatDiffersByEvenOneByteDoesNotHoldTheCommittedBlob(t *testing.T) {
	cases := map[string]string{
		"a changed byte":                   strings.Replace(committedBytes, "schema_version", "schema_versioN", 1),
		"a longer file":                    committedBytes + " ",
		"a shorter file":                   committedBytes[:len(committedBytes)-1],
		"line endings a checkout converts": strings.ReplaceAll(committedBytes, "\n", "\r\n"),
		"an empty file":                    "",
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			repo, objectID := aCommittedBlob(t)
			if err := os.WriteFile(filepath.Join(repo, "cfg", "p.json"), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}

			if fileHoldsTheCommittedBlob(repo, "cfg/p.json", objectID) {
				t.Errorf("%s was taken for the committed blob", name)
			}
		})
	}
}

// Anything that cannot be decided counts as work in the file, because a replacement offered over a file that was not
// understood can destroy what is in it.
func TestWhatCannotBeDecidedIsNotTheCommittedBlob(t *testing.T) {
	cases := map[string]func(t *testing.T, repo string){
		"a link to a file with the committed bytes": func(t *testing.T, repo string) {
			elsewhere := filepath.Join(repo, "elsewhere.json")
			if err := os.WriteFile(elsewhere, []byte(committedBytes), 0o644); err != nil {
				t.Fatal(err)
			}
			replaceWithLink(t, filepath.Join(repo, "cfg", "p.json"), elsewhere)
		},
		"a directory":         func(t *testing.T, repo string) { replaceWithDirectory(t, filepath.Join(repo, "cfg", "p.json")) },
		"a file that is gone": func(t *testing.T, repo string) { _ = os.Remove(filepath.Join(repo, "cfg", "p.json")) },
	}
	for name, arrange := range cases {
		t.Run(name, func(t *testing.T) {
			repo, objectID := aCommittedBlob(t)
			arrange(t, repo)

			if fileHoldsTheCommittedBlob(repo, "cfg/p.json", objectID) {
				t.Errorf("%s was taken for the committed blob", name)
			}
		})
	}
}

func replaceWithLink(t *testing.T, path, target string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(filepath.Dir(path), target)
	if err != nil {
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

func TestAnObjectIdOfALengthNoHashIsKnownForIsNotTheCommittedBlob(t *testing.T) {
	repo, objectID := aCommittedBlob(t)

	if fileHoldsTheCommittedBlob(repo, "cfg/p.json", objectID[:20]) {
		t.Errorf("a truncated object id was taken for the committed blob")
	}
}

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

// A repository that names objects with SHA-256 gets the same answer.
func TestARepositoryThatNamesObjectsWithSha256GetsTheSameAnswer(t *testing.T) {
	repo := t.TempDir()
	if err := exec.Command("git", "-C", repo, "init", "-q", "--object-format=sha256").Run(); err != nil {
		t.Skipf("this git cannot make a sha256 repository: %v", err)
	}
	path := filepath.Join(repo, "p.json")
	if err := os.WriteFile(path, []byte(committedBytes), 0o644); err != nil {
		t.Fatal(err)
	}
	output, err := exec.Command("git", "-C", repo, "hash-object", "--no-filters", "p.json").Output()
	if err != nil {
		t.Fatal(err)
	}
	objectID := strings.TrimSpace(string(output))

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
