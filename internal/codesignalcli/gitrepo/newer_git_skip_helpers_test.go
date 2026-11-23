package gitrepo

import "testing"

// skipUnlessTheRunningGitCanBeToldNotToFetch skips a test whose real git must
// be 2.44 or later, which reads GIT_NO_LAZY_FETCH, when the git running the
// test is older. It never passes such a test without running it: CI's git is
// newer, and `mise run acceptance-ci-env` runs it under CI's pinned git.
func skipUnlessTheRunningGitCanBeToldNotToFetch(t *testing.T) {
	t.Helper()
	if supported, version := noLazyFetchSupported(RunBytes, t.TempDir()); !supported {
		t.Skipf("needs git 2.44 or later, which can be told not to fetch; this git is %s. `mise run acceptance-ci-env` runs it under CI's pinned git", version)
	}
}
