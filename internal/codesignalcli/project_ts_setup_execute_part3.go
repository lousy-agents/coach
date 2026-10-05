package codesignalcli

func (s *boundedOutputSink) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if remaining := s.limit - s.buf.Len(); remaining > 0 {
		if len(p) > remaining {
			s.buf.Write(p[:remaining])
		} else {
			s.buf.Write(p)
		}
	}
	return len(p), nil
}

// setupResidueChangedPaths returns the untracked, modified, and gitignored
// paths (`--ignored`, since a package-manager install typically leaves a
// gitignored node_modules/ partially populated) that `git status --porcelain
// -z` reports under workingDirectory, for AC-SET-7's "identify files that
// may have changed" disclosure after a failed setup. The read is scoped to
// workingDirectory with a trailing `-- .` pathspec, so unrelated dirt
// elsewhere in a larger repository (workingDirectory can be a package
// directory inside a monorepo) is never reported. Returned paths are
// repository-root-relative, not workingDirectory-relative -- that is simply
// what `git status` reports, and a caller printing a path next to
// workingDirectory must account for the difference (a monorepo package at
// packages/app reports "packages/app/node_modules/", not "node_modules/").
// This only ever reads: it never invokes `git reset`/`git clean`/`git
// checkout` or any other command that could mutate workingDirectory.
//
// The returned bool is SetupOutcome.ResidueUnknown (see its doc); it is true
// when the disclosure itself could not be produced -- workingDirectory is
// not inside a Git worktree, or the bounded git status call otherwise
// failed -- in which case the returned paths are a best-effort fallback
// (workingDirectory itself), not a real status read.
func setupResidueChangedPaths(workingDirectory string) ([]string, bool) {
	output, err := runGitBytesBounded(workingDirectory, maxSetupResidueGitBytes, maxSetupResidueGitStderr, setupResidueGitTimeout, "status", "--porcelain", "-z", "--ignored", "--", ".")
	if err != nil {
		return []string{workingDirectory}, true
	}
	return parseSetupResidueStatusPaths(output), false
}
