package gitrepo

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

const treeObjectName = "0123456789012345678901234567890123456789"

// answeringLsTree is a git that knows the revision and prints output for the one ls-tree question about a path.
// The object checks (cat-file) succeed, and any other question is an error that names it.
func answeringLsTree(output string) Runner {
	return func(_ string, args ...string) ([]byte, error) {
		switch args[0] {
		case "ls-tree":
			return []byte(output), nil
		case "cat-file":
			return nil, nil
		}
		return nil, fmt.Errorf("answeringLsTree: unexpected git question: %s", strings.Join(args, " "))
	}
}

// Two questions ask what a revision holds at a path, whether it is a file and what kind of entry it is, and they
// read the same ls-tree record. A record Coach cannot read must be refused by both: a project that cannot be
// read is not a project without a package.json, so the file question must not call it absent where the kind
// question refuses it.
var unreadableLsTreeOutputs = map[string]string{
	"a record for another path":                       "100644 blob " + treeObjectName + "\tother.json\x00",
	"a record git does not print":                     "not a record\x00",
	"a record without a tab":                          "100644 blob 0123 unexpected-record-without-tab\x00",
	"a record with a field more":                      "100644 blob " + treeObjectName + " extra\tpackage.json\x00",
	"a record with a field less":                      "100644 blob\tpackage.json\x00",
	"two records for the path":                        "100644 blob " + treeObjectName + "\tpackage.json\x00040000 tree " + treeObjectName + "\tpackage.json\x00",
	"a line git ends with a newline instead of a NUL": "100644 blob " + treeObjectName + "\tpackage.json\n",
}

func TestBlobEntryAtRevisionRefusesARecordItCannotRead(t *testing.T) {
	for name, output := range unreadableLsTreeOutputs {
		t.Run(name, func(t *testing.T) { requireBlobEntryRefused(t, answeringLsTree(output)) })
	}
}

func requireBlobEntryRefused(t *testing.T, run Runner) {
	t.Helper()
	entry, found, err := BlobEntryAtRevision(run, "repo", "HEAD", "package.json")
	var operational *OperationalError
	if !errors.As(err, &operational) || found || entry != (TreeEntry{}) {
		t.Errorf("BlobEntryAtRevision = %v, %v, %v; want an *OperationalError and no entry", entry, found, err)
	}
}

func TestFileExistsAtRevisionRefusesARecordItCannotRead(t *testing.T) {
	for name, output := range unreadableLsTreeOutputs {
		t.Run(name, func(t *testing.T) { requireFileExistsRefused(t, answeringLsTree(output)) })
	}
}

func requireFileExistsRefused(t *testing.T, run Runner) {
	t.Helper()
	exists, err := FileExistsAtRevision(run, "repo", "HEAD", "package.json")
	var operational *OperationalError
	if !errors.As(err, &operational) || exists {
		t.Errorf("FileExistsAtRevision = %v, %v; want an *OperationalError, and never an answer", exists, err)
	}
}

func TestTheFileAndKindQuestionsRefuseTheSameOutput(t *testing.T) {
	outputs := map[string]string{
		"nothing at the path": "",
		"a regular file":      "100644 blob " + treeObjectName + "\tpackage.json\x00",
		"a directory":         "040000 tree " + treeObjectName + "\tpackage.json\x00",
		"a symbolic link":     "120000 blob " + treeObjectName + "\tpackage.json\x00",
	}
	for name, output := range unreadableLsTreeOutputs {
		outputs[name] = output
	}
	for name, output := range outputs {
		t.Run(name, func(t *testing.T) { requireTheSameVerdict(t, answeringLsTree(output)) })
	}
}

func requireTheSameVerdict(t *testing.T, run Runner) {
	t.Helper()
	_, _, blobErr := BlobEntryAtRevision(run, "repo", "HEAD", "package.json")
	_, kindErr := EntryKindAtRevision(run, "repo", "HEAD", "package.json")
	if (blobErr == nil) != (kindErr == nil) {
		t.Errorf("BlobEntryAtRevision error = %v, EntryKindAtRevision error = %v; both questions read one record and must refuse it together", blobErr, kindErr)
	}
}

func TestBlobEntryAtRevisionAsksForNulTerminatedRecords(t *testing.T) {
	var asked []string
	run := func(_ string, args ...string) ([]byte, error) {
		if args[0] == "ls-tree" {
			asked = args
		}
		return nil, nil
	}

	if _, _, err := BlobEntryAtRevision(run, "repo", "HEAD", "package.json"); err != nil {
		t.Fatalf("BlobEntryAtRevision: %v", err)
	}

	want := []string{"ls-tree", "-z", "--full-tree", "HEAD", "--", ":(top,literal)package.json"}
	if !reflect.DeepEqual(asked, want) {
		t.Errorf("ls-tree args = %v; want %v, the question EntryKindAtRevision asks, so one parser reads both answers", asked, want)
	}
}
