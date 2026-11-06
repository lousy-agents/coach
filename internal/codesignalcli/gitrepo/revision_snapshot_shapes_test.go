package gitrepo

import (
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

func TestExtractRevisionKeepsModesLinksSubmodulesAndSharedBlobs(t *testing.T) {
	repo := gitfixture.Init(t)
	writeFile(t, filepath.Join(repo, "bin", "run.sh"), "#!/bin/sh\n", 0o755)
	writeFile(t, filepath.Join(repo, "dir with space", "f g.go"), "package f\n", 0o644)
	writeFile(t, filepath.Join(repo, "same-a.txt"), "shared\n", 0o644)
	writeFile(t, filepath.Join(repo, "nested", "same-b.txt"), "shared\n", 0o644)
	if err := os.Symlink("bin/run.sh", filepath.Join(repo, "link")); err != nil {
		t.Fatal(err)
	}
	gitWithIdentity(t, repo, "add", "-A")
	gitWithIdentity(t, repo, "commit", "-q", "-m", "files")
	submodule := strings.TrimSpace(gitWithIdentity(t, repo, "rev-parse", "HEAD"))
	gitWithIdentity(t, repo, "update-index", "--add", "--cacheinfo", "160000,"+submodule+",sub")
	gitWithIdentity(t, repo, "commit", "-q", "-m", "submodule")

	dir, err := ExtractRevision(repo, "HEAD")
	if err != nil {
		t.Fatalf("ExtractRevision: %v", err)
	}
	defer os.RemoveAll(dir)

	assertSnapshotShape(t, dir)
}

// A tree object can name a path that leaves the snapshot, which git itself refuses to check out. Writing
// it must fail and leave nothing outside the snapshot directory.
func TestExtractRevisionRefusesATreeThatNamesAPathOutsideTheSnapshot(t *testing.T) {
	repo := gitfixture.Init(t)
	escaped := "coach-escape-" + filepath.Base(t.TempDir())
	blob := strings.TrimSpace(gitWithIdentity(t, repo, "hash-object", "-w", "/dev/null"))
	inner := writeRawTree(t, repo, "100644 "+escaped+"\x00", blob)
	outer := writeRawTree(t, repo, "40000 ..\x00", inner)
	commit := strings.TrimSpace(gitWithIdentity(t, repo, "commit-tree", outer, "-m", "escape"))

	dir, err := ExtractRevision(repo, commit)

	if err == nil {
		os.RemoveAll(dir)
		t.Fatalf("ExtractRevision of a tree naming ../%s succeeded", escaped)
	}
	if _, statErr := os.Stat(filepath.Join(os.TempDir(), escaped)); statErr == nil {
		os.Remove(filepath.Join(os.TempDir(), escaped))
		t.Errorf("a file was written outside the snapshot directory")
	}
}

// writeRawTree writes a tree object holding one entry, given as "<mode> <name>\x00", for the object id.
func writeRawTree(t *testing.T, repo, entry, objectID string) string {
	t.Helper()
	raw, err := hex.DecodeString(objectID)
	if err != nil {
		t.Fatal(err)
	}
	cmd := gitCommand(t.Context(), repo, "hash-object", "-t", "tree", "-w", "--stdin", "--literally")
	cmd.Stdin = strings.NewReader(entry + string(raw))
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("hash-object: %v", err)
	}
	return strings.TrimSpace(string(output))
}

func TestExtractRevisionReportsTheObjectsAPartialCloneLacksAndFetchesNone(t *testing.T) {
	skipIfGitCannotRefuseLazyFetch(t)
	marker := sshMarker(t)
	clone, _ := partialClone(t, func(string) string { return "ssh://example.invalid/repo.git" })
	before := packFiles(t, clone)

	dir, err := ExtractRevision(clone, "HEAD")

	var absent *ObjectsNotLocalError
	if err == nil || !errors.As(err, &absent) {
		os.RemoveAll(dir)
		t.Fatalf("ExtractRevision = %v; want an ObjectsNotLocalError", err)
	}
	if absent.Count != 1 || absent.FirstPath != "cfg/p.json" {
		t.Errorf("error = %+v; want the one missing object, at cfg/p.json", absent)
	}
	if !strings.Contains(err.Error(), "git rev-list --objects --missing=print --no-walk") {
		t.Errorf("error %q; want it to name the batch that fetches the revision's files", err)
	}
	if _, statErr := os.Stat(marker); statErr == nil {
		t.Errorf("git ran the ssh program to fetch an object")
	}
	if after := packFiles(t, clone); after != before {
		t.Errorf("the object store went from %d to %d pack files", before, after)
	}
}
