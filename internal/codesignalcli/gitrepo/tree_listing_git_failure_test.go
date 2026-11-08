package gitrepo

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestAGitThatDoesNotAnswerInTimeIsABoundError(t *testing.T) {
	stalled := listingScript("sleep 120")

	err := streamListingBy(stalled, "", 300*time.Millisecond, []string{"ls-tree"}, func([]byte) error { return nil })

	var bound *BoundError
	if !errors.As(err, &bound) || bound.Kind != BoundTimeout || !strings.Contains(err.Error(), "git ls-tree") {
		t.Errorf("streamListing = %v; want a timeout BoundError that names git ls-tree", err)
	}
}

func TestAGitThatFailsIsReportedWithWhatItSaidOnStderrHeldToTheBudget(t *testing.T) {
	failing := listingScript("printf 'whole\\000'; head -c 200000 /dev/zero | tr '\\000' 'e' >&2; exit 3")

	err := streamListingBy(failing, "", time.Minute, []string{"ls-tree"}, func([]byte) error { return nil })

	if err == nil || !strings.Contains(err.Error(), "exit status 3") || !strings.Contains(err.Error(), "eeee") {
		t.Fatalf("streamListing = %v; want git's exit status and what it said", err)
	}
	if len(err.Error()) > gitStderrBudget+200 {
		t.Errorf("the error is %d bytes; what git said on stderr is held to %d", len(err.Error()), gitStderrBudget)
	}
}
