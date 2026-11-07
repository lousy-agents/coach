package gitrepo

import "testing"

// The batch for several revisions lists them together, so that one paste brings in what the whole scan reads.
func TestTheFetchAdviceForSeveralRevisionsListsThemInOneBatch(t *testing.T) {
	repository := fakeRepository{promisors: promisorOrigin}

	got := FetchAdviceFor(repository.run, "repo", "abc123", "HEAD")

	want := "fetch the revision's files with " + batchOf("origin", "{ git rev-list --objects --missing=print --no-walk abc123; git rev-list --objects --missing=print --no-walk HEAD; }") + " and run again"
	if got != want {
		t.Errorf("advice %q; want %q", got, want)
	}
}

func TestTheFetchAdviceForSeveralRevisionsIsWithheldWhenAnyNameCannotBePasted(t *testing.T) {
	repository := fakeRepository{promisors: promisorOrigin}

	for _, revisions := range [][]string{{"abc123", "--upload-pack=touch x"}, {"abc\x1b123", "HEAD"}} {
		if got := FetchAdviceFor(repository.run, "repo", revisions...); got != "" {
			t.Errorf("advice %q for %q; want none: a name git would read as an option or a terminal would act on", got, revisions)
		}
	}
}
