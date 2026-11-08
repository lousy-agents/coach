package projectconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

// aConvertedCheckout is a repository with the policy committed with LF line endings, and the file in the working tree
// as git writes it where it converts them. It returns the repository and the committed object id.
func aConvertedCheckout(t *testing.T, config ...string) (repo, objectID string) {
	t.Helper()
	repo, objectID = aCommittedBlob(t)
	for i := 0; i+1 < len(config); i += 2 {
		gitfixture.Git(t, repo, "config", config[i], config[i+1])
	}
	if err := os.Remove(filepath.Join(repo, "cfg", "p.json")); err != nil {
		t.Fatal(err)
	}
	gitfixture.Git(t, repo, "checkout", "--", "cfg/p.json")
	return repo, objectID
}

func writeAttributes(t *testing.T, repo, attributes string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo, ".gitattributes"), []byte(attributes), 0o644); err != nil {
		t.Fatal(err)
	}
}

func requireConverted(t *testing.T, repo, objectID string, want bool) {
	t.Helper()
	if got := holdsTheCommittedBlobConverted(NewGitRunner(), repo, "cfg/p.json", objectID); got != want {
		t.Errorf("holdsTheCommittedBlobConverted = %v; want %v", got, want)
	}
}

func TestACheckoutThatConvertedLineEndingsIsTheCommittedBlobConverted(t *testing.T) {
	repo, objectID := aConvertedCheckout(t, "core.autocrlf", "true")
	content, err := os.ReadFile(filepath.Join(repo, "cfg", "p.json"))
	if err != nil || !strings.Contains(string(content), "\r\n") {
		t.Fatalf("the checkout did not convert line endings: %q, %v", content, err)
	}

	requireConverted(t, repo, objectID, true)
	if fileHoldsTheCommittedBlob(repo, "cfg/p.json", objectID) {
		t.Errorf("the converted bytes were taken for the blob's own: the conversion is what the test is about")
	}
}

// The attributes convert as well as the setting does.
func TestACheckoutThatAnEolAttributeConvertedIsTheCommittedBlobConverted(t *testing.T) {
	repo, objectID := aCommittedBlob(t)
	writeAttributes(t, repo, "*.json text eol=crlf\n")
	if err := os.Remove(filepath.Join(repo, "cfg", "p.json")); err != nil {
		t.Fatal(err)
	}
	gitfixture.Git(t, repo, "checkout", "--", "cfg/p.json")

	requireConverted(t, repo, objectID, true)
}

func TestAConvertedCheckoutWithAnEditIsNotTheCommittedBlob(t *testing.T) {
	repo, objectID := aConvertedCheckout(t, "core.autocrlf", "true")
	edited := strings.ReplaceAll(strings.Replace(committedBytes, "roots", "other", 1), "\n", "\r\n")
	if err := os.WriteFile(filepath.Join(repo, "cfg", "p.json"), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}

	requireConverted(t, repo, objectID, false)
}
