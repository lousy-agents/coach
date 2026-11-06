package projectconfig

import (
	"errors"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

const nonFilePath = "cfg/p.json"

// committedAs makes a repository whose HEAD records something at nonFilePath that is not a file.
type committedAs func(t *testing.T, repo string)

func aLink(t *testing.T, repo string) { gitfixture.CommitLink(t, repo, nonFilePath, "real.json") }
func aDirectory(t *testing.T, repo string) {
	gitfixture.CommitFile(t, repo, nonFilePath+"/inner.json", "{}")
}
func aSubmodule(t *testing.T, repo string) { gitfixture.CommitGitlink(t, repo, nonFilePath) }

// nonFileWant is how a policy committed as something else is named: by what it is, with no way out.
type nonFileWant struct {
	commit committedAs
	cause  string // the stable word, spelled as the docs spell it
	named  string // what the message calls it
}

func nonFileRows() map[string]nonFileWant {
	return map[string]nonFileWant{
		"a symbolic link":   {aLink, "committed_as_link", "symbolic link"},
		"a directory":       {aDirectory, "directory", "directory"},
		"a submodule entry": {aSubmodule, "in_submodule", "submodule"},
	}
}

// What the revision holds at the path is read before the content is, so a link or a directory is
// named as one and not as a document that is not valid JSON, and a new policy is not offered over
// it: the committed entry would still be there.
func TestAPolicyCommittedAsSomethingOtherThanAFileIsNamedAsThat(t *testing.T) {
	for name, row := range nonFileRows() {
		t.Run(name+", read by a scan", func(t *testing.T) { requireNamedAsNotAFile(t, row, readByAScan) })
		t.Run(name+", read for readiness", func(t *testing.T) { requireNamedAsNotAFile(t, row, readForReadiness) })
	}
}

func readByAScan(repo string) error {
	_, err := Load(repo, "HEAD", nonFilePath)
	return err
}

func readForReadiness(repo string) error {
	_, err := LoadForReadiness(repo, "HEAD", nonFilePath)
	return err
}

func requireNamedAsNotAFile(t *testing.T, row nonFileWant, read func(repo string) error) {
	t.Helper()
	repo := committedRepository(t)
	row.commit(t, repo)

	var found *ConfigError
	if err := read(repo); !errors.As(err, &found) {
		t.Fatalf("error = %v (%T), want a *ConfigError", err, err)
	}

	if found.Kind != KindInvalid || found.Remedy != projectreadiness.RemedyNone || string(found.PolicyCause) != row.cause {
		t.Errorf("kind %d, remedy %d, cause %q; want an invalid policy with no remedy and cause %q", found.Kind, found.Remedy, found.PolicyCause, row.cause)
	}
	if !strings.Contains(found.Message, row.named) || strings.Contains(found.Message, "not valid JSON") {
		t.Errorf("message %q: it must call the entry a %s and not blame the content", found.Message, row.named)
	}
}

// A file at the path is a file whatever its mode says about being run.
func TestAPolicyCommittedAsAFileIsReadAsOneAndAnEmptyPathIsNotFound(t *testing.T) {
	repo := committedRepository(t)
	gitfixture.CommitFile(t, repo, nonFilePath, `{"schema_version":"1","roots":["."]}`)

	if _, err := Load(repo, "HEAD", nonFilePath); err != nil {
		t.Errorf("Load of a committed file = %v, want it read", err)
	}
	var found *ConfigError
	if _, err := Load(repo, "HEAD", "cfg/none.json"); !errors.As(err, &found) || found.Kind != KindNotFound {
		t.Errorf("Load of nothing = %v, want it not found", err)
	}
}

// A question git cannot answer is not answered by guessing that the entry is a file: a scan says
// it could not check what the revision holds and offers nothing, and readiness fails closed with
// the operational error for the same question.
func TestAnUnanswerableEntryKindIsNotGuessedAtByEitherRead(t *testing.T) {
	repo := committedRepository(t)
	gitfixture.CommitFile(t, repo, nonFilePath, `{"schema_version":"1","roots":["."]}`)
	gitAnswersNothing(t)

	var found *ConfigError
	if _, err := Load(repo, "HEAD", nonFilePath); !errors.As(err, &found) ||
		found.Remedy != projectreadiness.RemedyNone || found.PolicyCause != projectreadiness.CauseGitUnreadable ||
		!strings.Contains(found.Message, "what the revision holds at the path") {
		t.Errorf("Load = %v, want a read that could not check what the revision holds and offers nothing", err)
	}
	var operational *gitrepo.OperationalError
	if _, err := LoadForReadiness(repo, "HEAD", nonFilePath); !errors.As(err, &operational) ||
		!strings.Contains(operational.Message, "could not be resolved") {
		t.Errorf("LoadForReadiness = %v (%T), want an *OperationalError for the question about the entry", err, err)
	}
}

// gitAnswersNothing makes every git read through the Load seam fail, so what a read reports is
// what it said about the first question it asked.
func gitAnswersNothing(t *testing.T) {
	t.Helper()
	original := runProjectConfigGit
	t.Cleanup(func() { runProjectConfigGit = original })
	runProjectConfigGit = func(string, ...string) ([]byte, error) { return nil, errBoom }
}

// An entry of a kind git records but this version has no name for is still not a file, so it is
// refused as one rather than read.
func TestAnEntryOfAnotherKindIsRefusedAsNotARegularFile(t *testing.T) {
	record := "100644 tag 0123456789012345678901234567890123456789\t" + nonFilePath + "\x00"
	answer := func(string, ...string) ([]byte, error) { return []byte(record), nil }

	found, err := NonFileAtRevision(answer, "repo", "HEAD", nonFilePath)

	if err != nil || found == nil || found.PolicyCause != projectreadiness.CauseNotARegularFile || found.Remedy != projectreadiness.RemedyNone {
		t.Errorf("NonFileAtRevision = %+v, %v; want a refusal with cause %q and no remedy", found, err, projectreadiness.CauseNotARegularFile)
	}
}
