package projectconfig

// Selection is the policy document Discover resolved for one revision.
// An empty Path with a nil Fatal means the scan stays file-local.
// Fatal is set only when CanonicalPath exists at the revision and Load
// rejected it; callers must not fall back to DefaultPath in that case.
type Selection struct {
	Path                 string
	Deprecated           bool
	UncommittedCanonical bool
	// UnusableCanonical is why the working tree's entry at the canonical path cannot be the policy, when
	// something is there that is not a regular file the process can read. The scan does not read it either way:
	// a discovered policy is the revision's.
	UnusableCanonical *ConfigError
	// HeldCanonical is why nothing can be put at the canonical path although nothing is on disk there: the index
	// holds a staged copy or edit, a conflict, or a mark for it, or the path lies in a submodule. It is set only
	// where a move of project.json to that path could be offered, which is when project.json is the policy.
	HeldCanonical *ConfigError
	Fatal         error
}

// CanonicalPathTaken is whether the default policy's path holds anything that a policy put there would meet: a regular
// file that is not committed, an entry on disk that cannot be the policy, or what the index or a submodule holds for
// it when nothing is on disk. A policy cannot be moved or copied to a path that is taken, so whatever offers that move
// asks this one question and cannot disagree with the diagnostics, or with the check, which asks
// ConfigError.FreeForANewPolicy of the same path.
func (s Selection) CanonicalPathTaken() bool {
	return s.UncommittedCanonical || s.UnusableCanonical != nil || s.HeldCanonical != nil
}
