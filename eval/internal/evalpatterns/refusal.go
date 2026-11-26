package evalpatterns

import (
	"fmt"
	"regexp"
	"strings"
)

// Refusal is an entry the pattern extractor will not read, with the exit
// code it gives.
type Refusal struct {
	Label  string
	Exit   int
	Reason string
}

func (r Refusal) Error() string {
	return fmt.Sprintf("reference %q: %s", r.Label, r.Reason)
}

var shaPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

// Check refuses an entry that has no licence, is not pinned to a full
// commit SHA, or is not Go.
func (r Reference) Check() error {
	switch {
	case strings.TrimSpace(r.Licence) == "":
		return r.refuse("licence is required")
	case !shaPattern.MatchString(r.SHA):
		return r.refuse(fmt.Sprintf("sha %q is not a 40-hex commit SHA", r.SHA))
	case r.Language != "go":
		return r.refuse(fmt.Sprintf("language %q is not go", r.Language))
	}
	return nil
}

func (r Reference) refuse(reason string) error {
	return Refusal{Label: r.Label, Exit: InputInvalid, Reason: reason}
}
