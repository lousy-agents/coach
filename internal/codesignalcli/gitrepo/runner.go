package gitrepo

import "time"

// Runner runs git in dir with args and returns its standard output. The reads
// that depend on what git says (the tree, the index, the working-tree status)
// take one, so a caller picks the limits a read is held to and a test supplies
// canned output without a repository.
type Runner func(dir string, args ...string) ([]byte, error)

// BoundedRunner is a Runner that holds every run to the given output caps and
// wall-time limit, as RunBytesBounded does.
func BoundedRunner(maxStdout, maxStderr int64, timeout time.Duration) Runner {
	return func(dir string, args ...string) ([]byte, error) {
		return RunBytesBounded(dir, maxStdout, maxStderr, timeout, args...)
	}
}
