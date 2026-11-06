package projectconfig

import (
	"strings"
	"testing"
)

// A listing line the parser cannot read is an error, never a guess, so it can
// neither be taken for a staged copy to restore nor for the absence of one.
func TestAListingGitDoesNotPrintAsExpectedNeverProducesARestoreCommand(t *testing.T) {
	for _, c := range garbledListingCases() {
		t.Run(c.label, func(t *testing.T) {
			assertNoRestoreCommand(t, c.listing, c.committed)
		})
	}
}

type garbledListingCase struct {
	label     string
	listing   string
	committed bool
}

func garbledListingCases() []garbledListingCase {
	garbled := []struct{ name, listing string }{
		{"a missing field", "H 100644 " + committedBlob + "\tpol.json\x00"},
		{"an object id that is garbled", "H 100644 zzzzzzzz" + committedBlob[8:] + " 0\tpol.json\x00"},
		{"a stage that is garbled", "H 100644 " + committedBlob + " x\tpol.json\x00"},
		{"a mode that is garbled", "H 1006 " + committedBlob + " 0\tpol.json\x00"},
	}
	var cases []garbledListingCase
	for _, g := range garbled {
		cases = append(cases,
			garbledListingCase{g.name + ", committed", g.listing, true},
			garbledListingCase{g.name + ", never committed", g.listing, false})
	}
	return cases
}

func assertNoRestoreCommand(t *testing.T, listing string, committed bool) {
	t.Helper()
	status := explicitStatusFor(t, indexFake{uncommitted: !committed, lsFiles: listing})

	if strings.Contains(status.Message, "git restore") {
		t.Errorf("message %q; a listing that cannot be read must not produce a restore command", status.Message)
	}
}
