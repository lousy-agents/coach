package gitrepo

import (
	"strings"
	"testing"
)

// A line of the probe is a question mark and an object id: 40 hex digits for SHA-1, 64 for SHA-256. Anything
// else on the output is a git that did not do as asked.
func TestOnlyAQuestionMarkAndAnObjectIDIsAMissingObjectLine(t *testing.T) {
	sha1, sha256 := strings.Repeat("a", 40), strings.Repeat("b", 64)
	for _, line := range []string{"?" + sha1, "?" + sha256, "?" + strings.Repeat("0123456789abcdef", 3)[:48]} {
		if !missingObjectLine.MatchString(line) {
			t.Errorf("%q is not taken for a missing object", line)
		}
	}
	for _, line := range []string{sha1, "?" + sha1[:39], "?" + strings.Repeat("a", 65), "?" + strings.ToUpper(sha1), "?" + sha1 + " ", " ?" + sha1, "?" + sha1 + "x", "!" + sha1, ""} {
		if missingObjectLine.MatchString(line) {
			t.Errorf("%q is taken for a missing object", line)
		}
	}
}
