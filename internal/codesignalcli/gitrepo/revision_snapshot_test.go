package gitrepo

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

func gitWithIdentity(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = gitfixture.CommitEnv
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, output)
	}
	return string(output)
}

func writeFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

// repositoryWhoseAttributesAskForConversion commits files that `git archive` would change on the way out:
// a smudge filter named by the attributes and defined in the repository's own configuration, line endings,
// $Format$ substitution, and a file marked export-ignore. The returned marker is left by the smudge filter.
func repositoryWhoseAttributesAskForConversion(t *testing.T) (repo, marker string) {
	t.Helper()
	repo = gitfixture.Init(t)
	marker = filepath.Join(t.TempDir(), "smudged")
	writeFile(t, filepath.Join(repo, ".gitattributes"), "*.filtered filter=evil\n*.crlf text eol=crlf\nsubst.txt export-subst\nhidden.go export-ignore\n", 0o644)
	writeFile(t, filepath.Join(repo, "a.filtered"), "raw content\n", 0o644)
	writeFile(t, filepath.Join(repo, "lines.crlf"), "one\ntwo\n", 0o644)
	writeFile(t, filepath.Join(repo, "subst.txt"), "commit $Format:%H$\n", 0o644)
	writeFile(t, filepath.Join(repo, "hidden.go"), "package hidden\n", 0o644)
	gitWithIdentity(t, repo, "add", "-A")
	gitWithIdentity(t, repo, "commit", "-q", "-m", "files")
	gitWithIdentity(t, repo, "config", "filter.evil.smudge", "sh -c 'touch "+marker+"; sed s/raw/CONVERTED/'")
	return repo, marker
}

func TestExtractRevisionWritesTheRawBlobsWhateverTheCommittedAttributesAsk(t *testing.T) {
	repo, marker := repositoryWhoseAttributesAskForConversion(t)

	dir, err := ExtractRevision(repo, "HEAD")
	if err != nil {
		t.Fatalf("ExtractRevision: %v", err)
	}
	defer os.RemoveAll(dir)

	if _, err := os.Stat(marker); err == nil {
		t.Errorf("a smudge filter ran while the snapshot was read: a read-only scan must run no program the repository names")
	}
	for _, name := range []string{"a.filtered", "lines.crlf", "subst.txt", "hidden.go"} {
		assertSnapshotFileIsTheBlob(t, repo, dir, name)
	}
}

func assertSnapshotFileIsTheBlob(t *testing.T, repo, dir, name string) {
	t.Helper()
	want := gitWithIdentity(t, repo, "cat-file", "blob", "HEAD:"+name)
	got, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Errorf("%s is not in the snapshot: %v", name, err)
		return
	}
	if !bytes.Equal(got, []byte(want)) {
		t.Errorf("%s in the snapshot = %q; want the committed blob %q", name, got, want)
	}
}
