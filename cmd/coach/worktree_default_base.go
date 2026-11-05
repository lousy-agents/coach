package main

// withWorktreeDefaultBase makes a bare --worktree compare the working tree with
// HEAD, as the product contract for a scan with no explicit mode.
func withWorktreeDefaultBase(f codesignalFlags) codesignalFlags {
	if f.worktree && !f.baseline && f.base == "" {
		f.base = "HEAD"
	}
	return f
}
