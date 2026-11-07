package gitrepo

import (
	"os"
	"strings"
	"testing"
)

func TestAnOlderGitRefusesAPartialCloneWhosePromisorSettingHasNoValue(t *testing.T) {
	clone := filteredClone(t, twoCommitSource(t), false)
	config, err := os.ReadFile(clone + "/.git/config")
	if err != nil {
		t.Fatal(err)
	}
	valueless := strings.Replace(string(config), "\tpromisor = true\n", "\tpromisor\n", 1)
	if valueless == string(config) {
		t.Fatal("the clone's configuration did not hold the setting this test rewrites")
	}
	if err := os.WriteFile(clone+"/.git/config", []byte(valueless), 0o644); err != nil {
		t.Fatal(err)
	}

	err = refuseWhenReadRevisionsLackObjects(RunBytes, clone, "", olderThanTheInstruction)

	if !strings.Contains(refusalText(err), "HEAD has 2 objects that are not in it") {
		t.Errorf("RefuseMissingObjects = %v; want the clone recognised as a partial clone and refused", err)
	}
}

// A remote's name may hold a dot, and the setting git wrote for it is the only marker a current git leaves.
func TestAnOlderGitRefusesAPartialCloneWhosePromisorRemoteHasADotInItsName(t *testing.T) {
	clone := filteredClone(t, twoCommitSource(t), false)
	gitOutput(t, clone, "remote", "rename", "origin", "my.remote")

	err := refuseWhenReadRevisionsLackObjects(RunBytes, clone, "", olderThanTheInstruction)

	if !strings.Contains(refusalText(err), "HEAD has 2 objects that are not in it") || !strings.Contains(refusalText(err), "git fetch my.remote") {
		t.Errorf("RefuseMissingObjects = %v; want the dotted remote recognised, refused and named in the fetch", err)
	}
}

func TestAnOlderGitRefusesWhenItCannotTellWhichObjectsAreMissing(t *testing.T) {
	clone := filteredClone(t, twoCommitSource(t), true)

	err := refuseWhenReadRevisionsLackObjects(RunBytes, clone, "no-such-ref", olderThanTheInstruction)

	if !strings.Contains(refusalText(err), "could not tell whether this repository has the objects Coach reads") {
		t.Errorf("RefuseMissingObjects = %v; want a refusal for want of an answer, not a read on a guess", err)
	}
}
