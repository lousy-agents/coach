package codesignalcli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/lousy-agents/coach/pkg/projectmodel"

	"strings"
)

// checkOutputParents walks every existing parent component of cleanOutput
// (relative to repositoryRootDir) via os.Lstat -- not os.Stat -- so a
// symlinked parent directory is rejected rather than silently followed. A
// missing parent is also rejected: a create-only write must never mkdir -p.
func checkOutputParents(repositoryRootDir, cleanOutput string) error {
	segments := strings.Split(filepath.ToSlash(cleanOutput), "/")
	current := repositoryRootDir
	for i := 0; i < len(segments)-1; i++ {
		current = filepath.Join(current, segments[i])
		info, err := os.Lstat(current)
		if err != nil {
			return fmt.Errorf("parent directory %q does not exist", strings.Join(segments[:i+1], "/"))
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("parent path component %q is a symlink", strings.Join(segments[:i+1], "/"))
		}
		if !info.IsDir() {
			return fmt.Errorf("parent path component %q is not a directory", strings.Join(segments[:i+1], "/"))
		}
	}
	return nil
}

// writeSuggestOutput performs the authoritative create-only write: an
// O_EXCL open that fails with fs.ErrExist when the target already exists.
// For the batch --suggest-project-config path specifically, this is the only
// existence check for --output -- there is deliberately no preflight stat,
// both because issue #220 puts "target already exists" last in the failure
// precedence (after root discovery) and because O_EXCL leaves no TOCTOU
// window: a concurrently created target is never clobbered, and a symlink at
// the target position is never followed.
//
// A failed Write or Close (ENOSPC, EIO, ...) leaves target removed: the
// O_EXCL create already succeeded, so without this cleanup a truncated file
// would remain at target and make the next run fail with
// SuggestDiagOutputExists instead of surfacing the original write failure
// again. Removal is best-effort -- its own error is never propagated, and
// the original writeErr/closeErr is always what is returned.
func writeSuggestOutput(repositoryRootDir, cleanOutput string, candidate []byte) (exists bool, err error) {
	target := filepath.Join(repositoryRootDir, filepath.FromSlash(cleanOutput))
	file, openErr := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if openErr != nil {
		if errors.Is(openErr, fs.ErrExist) {
			return true, nil
		}
		return false, unwrapPathError(cleanOutput, openErr)
	}
	_, writeErr := file.Write(candidate)
	closeErr := file.Close()
	if writeErr != nil {
		os.Remove(target)
		return false, unwrapPathError(cleanOutput, writeErr)
	}
	if closeErr != nil {
		os.Remove(target)
		return false, unwrapPathError(cleanOutput, closeErr)
	}
	return false, nil
}

// writeSuggestCandidate performs the post-discovery create-only --output
// write. An ordinary write failure (read-only directory, out of disk space,
// name too long) is SuggestDiagOutputInvalid; only discovery-result
// serialization maps to SuggestDiagFailed.
func writeSuggestCandidate(root, cleanOutput, outputPath, revisionSHA string, result projectmodel.RootDiscoveryResult, candidate []byte) (fail SuggestionResult, ok bool) {
	exists, writeErr := writeSuggestOutput(root, cleanOutput, candidate)
	if exists {
		return suggestFailureAfterDiscovery(revisionSHA, result, SuggestDiagOutputExists, outputPath, "coach codesignal --suggest-project-config: --output target already exists"), false
	}
	if writeErr != nil {
		return suggestFailureAfterDiscovery(revisionSHA, result, SuggestDiagOutputInvalid, outputPath, writeErr.Error()), false
	}
	return SuggestionResult{}, true
}
