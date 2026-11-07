package gitrepo

import (
	"errors"
	"strings"
	"testing"
)

// A git before 2.44 ignores the instruction not to fetch, and what it takes for a promisor remote is more than
// the settings Coach reads: a remote with remote.<name>.partialclonefilter is one to git, with promisor unset or
// false, and a read of a missing object fetches from it. So on such a git the configuration never decides whether
// a read may go ahead; the probe does, which lists what is missing and fetches nothing. The configuration only
// words the refusal.
func TestOnAnOldGitAMissingObjectIsRefusedWhateverTheConfigurationSays(t *testing.T) {
	cases := []struct {
		name       string
		repository fakeRepository
	}{
		{"a remote with a partial clone filter and promisor false", fakeRepository{version: oldGit, promisors: "remote.origin.promisor false\nremote.origin.partialclonefilter blob:none\n"}},
		{"a remote with a partial clone filter and no promisor setting", fakeRepository{version: oldGit, promisors: "remote.origin.partialclonefilter blob:none\n"}},
		{"no setting Coach reads at all", fakeRepository{version: oldGit}},
		{"a configuration git cannot be asked", fakeRepository{version: oldGit, configErr: errors.New("boom")}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertOldGitRefusesWhatIsMissing(t, c.repository)
		})
	}
}

func assertOldGitRefusesWhatIsMissing(t *testing.T, repository fakeRepository) {
	t.Helper()
	repository.probes = map[string]probeAnswer{"HEAD": {output: missingIDs(firstMissing)}}

	err := RefuseMissingObjects(repository.run, "repo", "")

	var refusal *OperationalError
	if !errors.As(err, &refusal) {
		t.Fatalf("RefuseMissingObjects = %v after probing %q; want a refusal: this git would fetch the object", err, repository.probed)
	}
	for _, word := range []string{"git 2.43.0 cannot be told not to fetch missing objects", "HEAD has 1 object that is not in it", "use git 2.44 or later"} {
		if !strings.Contains(err.Error(), word) {
			t.Errorf("refusal %q does not say %q", err, word)
		}
	}
}

// A repository the configuration does not call a partial clone is not called one in the words either, and one
// that it does is told its fetch.
func TestTheRefusalCallsTheRepositoryAPartialCloneOnlyWhenTheConfigurationDoes(t *testing.T) {
	probes := map[string]probeAnswer{"HEAD": {output: missingIDs(firstMissing)}}
	plain := fakeRepository{version: oldGit, probes: probes}
	partial := fakeRepository{version: oldGit, promisors: promisorOrigin, probes: probes}

	plainErr := RefuseMissingObjects(plain.run, "repo", "")
	partialErr := RefuseMissingObjects(partial.run, "repo", "")

	if plainErr == nil || strings.Contains(plainErr.Error(), "is a partial clone") {
		t.Errorf("refusal %v calls a repository a partial clone that the configuration does not", plainErr)
	}
	if partialErr == nil || !strings.Contains(partialErr.Error(), "is a partial clone") || !strings.Contains(partialErr.Error(), "git fetch origin") {
		t.Errorf("refusal %v does not call the partial clone one and offer its fetch", partialErr)
	}
}

// And a read of an old git that finds nothing missing goes ahead, whatever the configuration says.
func TestOnAnOldGitAReadWithNothingMissingGoesAhead(t *testing.T) {
	for _, repository := range []fakeRepository{
		{version: oldGit},
		{version: oldGit, promisors: promisorOrigin},
		{version: oldGit, promisors: "remote.origin.partialclonefilter blob:none\n"},
	} {
		if err := RefuseMissingObjects(repository.run, "repo", ""); err != nil {
			t.Errorf("RefuseMissingObjects(%+v) = %v; want a read with every object here to go ahead", repository, err)
		}
	}
}

func TestOnAnOldGitAnUnaskableProbeIsARefusalAndMissingGitIsSaidSo(t *testing.T) {
	unaskable := fakeRepository{version: oldGit, probes: map[string]probeAnswer{"HEAD": {err: errors.New("exit status 128")}}}
	if err := RefuseMissingObjects(unaskable.run, "repo", ""); err == nil || !strings.Contains(err.Error(), "could not tell whether") {
		t.Errorf("RefuseMissingObjects = %v; want a refusal that says the probe could not be answered", err)
	}
}
