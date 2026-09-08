package codesignalcli

import (
	"fmt"
	"strings"
)

// fileExistsAtRevision reports whether repoPath exists as a blob at
// revision, without reading its content. It reuses runProjectConfigGit's
// bounded git invocation rather than a bespoke exec call.
//
// The check runs in three steps because no single git-plumbing call
// unambiguously reports "this path is absent" separately from "this
// repository/revision/object could not be read":
//
//  1. Confirm revision itself resolves to a commit. Any failure here (bad
//     revision, unreadable repository, corrupt commit/tree objects) is
//     unambiguously operational.
//  2. Resolve repoPath within revision's tree via `git ls-tree --full-tree
//     <revision> -- <repoPath>`, which walks tree objects but never opens
//     blob content. `--full-tree` is required: without it, ls-tree's
//     pathspec is interpreted relative to dir (the process's cwd), not the
//     repository root, so a caller running from any subdirectory would get
//     a false "absent" result for a path that exists at revision -- the
//     same root-relative semantics loadProjectConfigForReadiness's
//     `git show <rev>:<path>` already has. Dropping `--full-tree` in a
//     future edit would silently reintroduce that false-negative readiness
//     verdict. The entry type must be blob: a tree (or gitlink) at the
//     same path is not the file the snapshot checks look for, and treating
//     it as present would let a directory named package.json pass
//     project_shape. This call distinguishes the remaining states by its
//     own exit code, not just its output: it exits 0 with empty stdout
//     when repoPath is genuinely absent from the tree, but exits non-zero
//     when a tree object along the path cannot be read (e.g. a corrupt or
//     missing subtree) -- a case `git rev-parse --verify <rev>:<path>`
//     cannot distinguish from "absent", and which must fail closed as an
//     operational error rather than silently report the path missing.
//  3. Confirm the resolved blob object is actually present and readable via
//     `git cat-file -e <sha>` on the concrete blob SHA. Step 2's `ls-tree`
//     only reads the tree entry recording the blob's SHA, not the blob
//     itself, so a corrupt/missing blob object still resolves a SHA there;
//     any failure here means the object store itself is unreadable, which
//     must also fail closed rather than report the path absent.
func fileExistsAtRevision(dir, revision, repoPath string) (bool, error) {
	if _, err := runProjectConfigGit(dir, "cat-file", "-e", revision+"^{commit}"); err != nil {
		return false, &OperationalError{Message: fmt.Sprintf("coach codesignal --check-project: revision %q could not be verified: %s", revision, err)}
	}

	output, err := runProjectConfigGit(dir, "ls-tree", "--full-tree", revision, "--", repoPath)
	if err != nil {
		return false, &OperationalError{Message: fmt.Sprintf("coach codesignal --check-project: %q could not be resolved at revision %q: %s", repoPath, revision, err)}
	}
	blobSHA, ok := lsTreeBlobSHA(output)
	if !ok {
		return false, nil
	}

	if _, err := runProjectConfigGit(dir, "cat-file", "-e", blobSHA); err != nil {
		return false, &OperationalError{Message: fmt.Sprintf("coach codesignal --check-project: %q could not be read at revision %q: %s", repoPath, revision, err)}
	}
	return true, nil
}

func lsTreeBlobSHA(output []byte) (string, bool) {
	trimmed := strings.TrimSpace(string(output))
	if trimmed == "" || strings.Contains(trimmed, "\n") {
		return "", false
	}
	meta, _, found := strings.Cut(trimmed, "\t")
	if !found {
		return "", false
	}
	fields := strings.Fields(meta)
	if len(fields) != 3 || fields[1] != "blob" {
		return "", false
	}
	return fields[2], true
}
