package gitrepo

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// Through the stream: the child prints the answer and the error, and exits 0, as git does.
func TestReadObjectsSeparatesADamagedObjectFromAMissingOne(t *testing.T) {
	withStreamChild(t, func(ctx context.Context) *exec.Cmd {
		return exec.CommandContext(ctx, "sh", "-c", "printf '"+damagedObject+" missing\\n"+absentObject+" missing\\n'; echo 'error: unable to unpack "+damagedObject+" header' >&2")
	})

	outcome, err := readObjects("", []string{damagedObject, absentObject}, snapshotBudget{wall: time.Minute, object: 1 << 20}, func(string, []byte) error { return nil })

	if err != nil || len(outcome.unreadable) != 1 || outcome.unreadable[0] != damagedObject || len(outcome.missing) != 1 || outcome.missing[0] != absentObject {
		t.Errorf("readObjects = %+v, %v; want the damaged object unreadable and the other missing", outcome, err)
	}
}

func TestADamagedObjectEndsTheSnapshotWithTheFirstPathAndNoFetchAdvice(t *testing.T) {
	withStreamChild(t, func(ctx context.Context) *exec.Cmd {
		return exec.CommandContext(ctx, "sh", "-c", "printf '"+damagedObject+" missing\\n'; echo 'error: unable to unpack "+damagedObject+" header' >&2")
	})
	entries := []snapshotEntry{{mode: "100644", oid: damagedObject, path: "cfg/p.json"}, {mode: "100644", oid: damagedObject, path: "other/p.json"}}

	err := writeSnapshotWithin("", t.TempDir(), entries, 1<<20)

	damaged, ok := err.(*ObjectsUnreadableError)
	if !ok || damaged.Count != 1 || damaged.FirstPath != "cfg/p.json" {
		t.Fatalf("writeSnapshotWithin = %v; want an ObjectsUnreadableError for one object whose first path is cfg/p.json", err)
	}
	if strings.Contains(err.Error(), "fetch") || strings.Contains(err.Error(), "not in the local object store") {
		t.Errorf("message %q offers a remedy for a missing object to a damaged one", err)
	}
}

func TestTheDamagedObjectsMessageCountsInTheGrammarOfTheNumber(t *testing.T) {
	one := (&ObjectsUnreadableError{Count: 1, FirstPath: "a.go", Said: "error: x"}).Error()
	many := (&ObjectsUnreadableError{Count: 3, FirstPath: "a.go", Said: "error: x"}).Error()

	if !strings.Contains(one, "1 object that git cannot read; it is in the object store") || !strings.Contains(many, "3 objects that git cannot read; they are in the object store") {
		t.Errorf("messages %q and %q do not agree with their counts", one, many)
	}
}
