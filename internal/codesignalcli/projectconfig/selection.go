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
	Fatal             error
}
