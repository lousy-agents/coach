package gitrepo

// UnexpectedOutputError is Coach's own finding that what git printed cannot be used: a line it
// cannot parse, or a record for a path other than the one asked about. Git reported no error, so a
// message that quotes it must not word it as something git said.
type UnexpectedOutputError struct {
	Finding string
}

func (e *UnexpectedOutputError) Error() string { return e.Finding }
