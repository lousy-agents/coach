package projectconfig

import (
	"strings"
	"testing"
)

// git on macOS lists a submodule under the precomposed spelling of its name, whichever form the name
// was asked for in, and that is still the submodule asked about.
func TestASubmoduleListedUnderThePrecomposedNameIsTheOneAskedFor(t *testing.T) {
	const decomposed = "vendor/subé"
	listing := "160000 " + committedBlob + " 0\tvendor/subé\x00"
	run := func(string, ...string) ([]byte, error) { return []byte(listing), nil }

	recorded, err := submoduleRecordedAs(run, "repo", decomposed)

	if err != nil || !strings.HasPrefix(recorded, "vendor/sub") {
		t.Errorf("submoduleRecordedAs = %q, %v; want the submodule git listed under the precomposed name", recorded, err)
	}
}

// A submodule of another name is not the one asked about, whatever form it is spelled in.
func TestASubmoduleOfAnotherNameIsNotTheOneAskedFor(t *testing.T) {
	listing := "160000 " + committedBlob + " 0\tvendor/subè\x00"
	run := func(string, ...string) ([]byte, error) { return []byte(listing), nil }

	recorded, err := submoduleRecordedAs(run, "repo", "vendor/subé")

	if err != nil || recorded != "" {
		t.Errorf("submoduleRecordedAs = %q, %v; want none: the listed name has another accent", recorded, err)
	}
}
