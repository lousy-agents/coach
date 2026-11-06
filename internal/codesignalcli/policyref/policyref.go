// Package policyref is the policy a run reads and who chose it. It is a plain
// value with no dependencies, so the packages that only print or carry a policy
// (the command phrase builder, the toolchain errors) do not have to import the
// package that loads one.
package policyref

// Ref is the policy a run reads and who chose it. The two travel together
// because they decide how the policy is judged: a policy the user named with
// --project-config must be a regular file on disk (ADR-008), while one Coach
// discovered is read from the analyzed revision. The zero value is no policy
// named and none discovered yet.
type Ref struct {
	// Path is repository-relative. It is empty for the zero value, and for a policy
	// named with an empty --project-config value, which flag validation rejects
	// before any run reads it: Explicit, not an empty Path, says the user named one.
	Path string
	// Explicit is true when the user named the policy with --project-config.
	Explicit bool
}

// Named is a policy the user named with --project-config.
func Named(path string) Ref {
	return Ref{Path: path, Explicit: true}
}

// Discovered is a policy Coach selected without being told which.
func Discovered(path string) Ref {
	return Ref{Path: path}
}

// NamedPath is the --project-config value to print or re-check, and false when
// the user did not name the policy. Anything printed from a failure uses this,
// never Path: naming a discovered policy with --project-config would make the
// next command judge it as a named one.
func (r Ref) NamedPath() (string, bool) {
	if !r.Explicit {
		return "", false
	}
	return r.Path, true
}

// JudgedOnDisk is true for a policy the user named, which must be a regular file
// on disk, and false for a discovered one, which is read at the analyzed revision.
func (r Ref) JudgedOnDisk() bool {
	return r.Explicit
}
