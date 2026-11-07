package projectconfig

import (
	"errors"
	"testing"
)

// A git that cannot say whether it ignores case has not said that two spellings are two paths, so the lookup is that
// failure and not a path outside every submodule.
func TestAGitThatCannotSayWhetherItIgnoresCaseIsAFailureOfTheLookup(t *testing.T) {
	git := &theIndexRecords{submodules: []string{"vendor/sub"}, configErr: errors.New("exit status 128: fatal: bad config line 1")}

	recorded, err := submoduleRecordedAs(git.run, "repo", "Vendor/Sub")

	if err == nil || recorded != "" {
		t.Errorf("submoduleRecordedAs = %q, %v; want the failure of the question", recorded, err)
	}
}

// The spelling asked for is preferred, and settles the lookup without asking whether git ignores case.
func TestASubmoduleRecordedInTheSpellingAskedForNeedsNoQuestionAboutCase(t *testing.T) {
	git := &theIndexRecords{submodules: []string{"vendor/Sub", "vendor/sub"}, configErr: errors.New("never asked")}

	recorded, err := submoduleRecordedAs(git.run, "repo", "vendor/sub")

	if err != nil || recorded != "vendor/sub" || git.askedAbout("config") {
		t.Errorf("submoduleRecordedAs = %q, %v, asked %v; want the exact spelling and no question about case", recorded, err, git.asked)
	}
}

// A path below the submodule is inside it: every prefix is looked up, and the submodule is named as the index records it.
func TestAPathBelowASubmoduleInAnotherCaseIsInsideTheSubmoduleNamedAsRecorded(t *testing.T) {
	git := &theIndexRecords{submodules: []string{"vendor/sub"}, ignoreCase: "true"}

	submodule, err := enclosingSubmodule(git.run, "repo", "Vendor/Sub/cfg/p.json")

	if err != nil || submodule != "vendor/sub" {
		t.Errorf("enclosingSubmodule = %q, %v; want the submodule as the index records it", submodule, err)
	}
}

func TestAPathBesideASubmoduleInAnotherCaseIsInsideNone(t *testing.T) {
	git := &theIndexRecords{submodules: []string{"vendor/sub"}, ignoreCase: "true"}

	submodule, err := enclosingSubmodule(git.run, "repo", "Vendor/Subtle/p.json")

	if err != nil || submodule != "" {
		t.Errorf("enclosingSubmodule = %q, %v; want none: Subtle is not Sub", submodule, err)
	}
}
