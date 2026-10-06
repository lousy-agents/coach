package gitrepo

// OperationalError signals a failure in the surrounding environment (not a
// Git worktree, an unresolvable revision, a missing git executable, or a
// malformed diff stream) rather than a per-file problem. Callers should map
// it to a single actionable message and a non-zero exit status.
type OperationalError struct {
	Message string
	// reason is Message's failure description with no absolute path
	// interpolated into it. It is set only for cases where Message does
	// embed a path; Reason falls back to Message otherwise. This lets a
	// caller that must never let an absolute path reach a diagnostic
	// (SuggestProjectConfig's snapshotUnavailableMessage) use a
	// structurally path-free string instead of scrubbing Message via
	// substring match, which is fragile against escaped/re-encoded forms
	// of the path.
	reason string
}

func (e *OperationalError) Error() string {
	return e.Message
}

// Reason reports e's failure description without any interpolated
// absolute path. See the reason field's doc comment for why this exists.
func (e *OperationalError) Reason() string {
	if e.reason != "" {
		return e.reason
	}
	return e.Message
}
