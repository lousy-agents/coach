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
		t.Run(name, func(t *testing.T) { requireTheEditIsNoticed(t, content) })
	}
}

func requireTheEditIsNoticed(t *testing.T, content string) {
	t.Helper()
	repo, objectID := aCommittedBlob(t)
	if err := os.WriteFile(filepath.Join(repo, "cfg", "p.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	if fileHoldsTheCommittedBlob(repo, "cfg/p.json", objectID) {
		t.Errorf("%q was taken for the committed blob", content)
	}
}

func TestAnObjectIdOfALengthNoHashIsKnownForIsNotTheCommittedBlob(t *testing.T) {
	repo, objectID := aCommittedBlob(t)

	if fileHoldsTheCommittedBlob(repo, "cfg/p.json", objectID[:20]) {
		t.Errorf("a truncated object id was taken for the committed blob")
	}
}
