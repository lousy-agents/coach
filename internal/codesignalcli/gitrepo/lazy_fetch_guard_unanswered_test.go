package gitrepo

import (
	"errors"
	"strings"
	"testing"
)

// A probe that fails says nothing about what is missing, and a read that might run the remote's transport
// is not made on a guess.
func TestRefuseMissingObjectsRefusesWhenItCannotTellWhatIsMissing(t *testing.T) {
	cases := []struct {
		name       string
		repository fakeRepository
		base       string
		cause      string
	}{
		{"the probe fails", fakeRepository{probes: map[string]probeAnswer{"HEAD": {err: errors.New("exit status 128: fatal: bad object HEAD")}}}, "", "bad object HEAD"},
		{"the probe runs out of time", fakeRepository{probes: map[string]probeAnswer{"HEAD": {err: &BoundError{Kind: BoundTimeout, message: "git execution timed out after 30s"}}}}, "", "timed out"},
		{"the probe prints what is not an object id", fakeRepository{probes: map[string]probeAnswer{"HEAD": {output: "0123456789012345678901234567890123456789 a.go\n"}}}, "", "a.go"},
		{"the probe prints a bare object id, which git prints only without --quiet", fakeRepository{probes: map[string]probeAnswer{"HEAD": {output: firstMissing + "\n"}}}, "", firstMissing},
		{"the probe prints an object id that something precedes", fakeRepository{probes: map[string]probeAnswer{"HEAD": {output: "x" + missingIDs(firstMissing)}}}, "", "x?"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertRefusedForWantOfAnAnswer(t, c.repository, c.base, c.cause)
		})
	}
}

func assertRefusedForWantOfAnAnswer(t *testing.T, repository fakeRepository, base, cause string) {
	t.Helper()
	repository.version, repository.promisors = oldGit, promisorOrigin

	err := RefuseMissingObjects(repository.run, "repo", base)

	var refusal *OperationalError
	if !errors.As(err, &refusal) || refusal.Cause == nil {
		t.Fatalf("RefuseMissingObjects = %v; want a refusal that keeps the cause", err)
	}
	for _, word := range []string{"could not tell whether", "partial clone", "git 2.43.0", "git 2.44 or later", cause} {
		if !strings.Contains(err.Error(), word) {
			t.Errorf("refusal %q does not say %q", err, word)
		}
	}
}
