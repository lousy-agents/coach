package gitrepo

// headResolvesThen is the fake's git, except that its first rev-parse, the guard's resolution of HEAD, answers with a
// commit and every later one fails with err: a question asked after the probes, not before them.
func headResolvesThen(err error, fallback Runner) Runner {
	resolutions := 0
	return func(dir string, args ...string) ([]byte, error) {
		if args[0] != "rev-parse" {
			return fallback(dir, args...)
		}
		resolutions++
		if resolutions > 1 {
			return nil, err
		}
		return []byte(firstMissing + "\n"), nil
	}
}
