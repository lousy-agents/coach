package gitfixture

// Git runs git with args in dir for a test that needs a repository state no other fixture builds, and
// returns what it printed. It fails the test when git fails.
func Git(t T, dir string, args ...string) string {
	t.Helper()
	return run(t, dir, nil, args...)
}
