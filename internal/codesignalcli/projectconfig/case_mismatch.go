package projectconfig

import (
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/shellword"
)

// caseMismatchError is a policy path with a component that the disk spells in
// another case than the path does. It carries both spellings instead of a
// finished message because what to do about it depends on which of them the
// analyzed revision holds, which only adviseOnRevision can ask.
type caseMismatchError struct {
	repoPath  string
	directory string
	typed     string
	onDisk    string
	corrected string
	// isFile is whether the differing component is the last: a file, which git mv can rename in case only.
	isFile bool
	// displaced is the error that was found first and that the mismatch was reported instead of: a policy
	// missing from a directory the disk spells in another case. It is nil when the policy is there. It is what
	// is reported when the revision holds the typed path, which settles the spelling.
	displaced error
}

// newCaseMismatch records that the component at index of parts is spelled
// onDisk on disk. The components above it already match, so they name the
// directory that holds it, and corrected is the path with the disk's spelling
// of that component and the path's own spelling of those below it.
func newCaseMismatch(repoPath string, parts []string, index int, onDisk string) caseMismatchError {
	corrected := slices.Clone(parts)
	corrected[index] = onDisk
	return caseMismatchError{
		repoPath:  repoPath,
		directory: strings.Join(parts[:index], "/"),
		typed:     parts[index],
		onDisk:    onDisk,
		corrected: strings.Join(corrected, "/"),
		isFile:    index == len(parts)-1,
	}
}

func (m caseMismatchError) Error() string {
	return m.config(nil).Message
}

// adviseOnRevision is err, with a case mismatch settled by the analyzed revision, and a policy missing from disk
// turned into a refusal when the revision holds its path in another case.
//
// The revision decides what the path names: one that holds exactly the path as typed is the policy that is read,
// whatever spelling a file system that folds case shows for the file, so the mismatch is dropped and what it
// displaced, the missing file or nothing, is reported. Otherwise the mismatch is refused with what the revision
// holds and the way out that follows. A revision that cannot be asked settles nothing, so it is refused.
func adviseOnRevision(err error, repoPath string, entries revisionEntries) error {
	var mismatch caseMismatchError
	if errors.As(err, &mismatch) {
		if holdsAsTyped(entries, repoPath) {
			return mismatch.displaced
		}
		return mismatch.config(entries)
	}
	var found *ConfigError
	if entries != nil && errors.As(err, &found) && found.AbsentFromDisk() {
		if respelled := respelledInRevision(entries, repoPath); respelled != nil {
			return respelled
		}
	}
	return err
}

// holdsAsTyped is whether the revision can be asked and holds every component of the path spelled as typed.
func holdsAsTyped(entries revisionEntries, repoPath string) bool {
	if entries == nil {
		return false
	}
	held, err := entries.holdsPath(repoPath)
	return err == nil && held
}

// config is the refusal for the mismatch. With no way to ask the revision, or
// when asking fails, it states the two spellings and gives no advice, because
// advice built from the user's own argument can point the wrong way.
func (m caseMismatchError) config(entries revisionEntries) *ConfigError {
	reason := fmt.Sprintf("is stored as %q on disk, not %q; this filesystem ignores case, so the name resolves, but the analyzed revision tells the two spellings apart", m.onDisk, m.typed)
	refusal := newUnusableOnDisk(m.repoPath, projectreadiness.CauseNameCaseDiffers, reason+m.revisionClause(entries))
	refusal.SpelledOnDisk = path.Join(m.directory, m.onDisk)
	return refusal
}

// revisionClause says which spelling the revision holds, and the one way out
// that follows: pass the disk's spelling when the revision holds that one,
// undo the rename when it holds the user's, and nothing when it holds both or
// neither, where no spelling is the right one to move to.
func (m caseMismatchError) revisionClause(entries revisionEntries) string {
	if entries == nil {
		return ""
	}
	typed, onDisk := path.Join(m.directory, m.typed), path.Join(m.directory, m.onDisk)
	heldTyped, typedErr := entries.holds(typed)
	heldOnDisk, onDiskErr := entries.holds(onDisk)
	if typedErr != nil || onDiskErr != nil {
		return ""
	}
	switch {
	case heldTyped && heldOnDisk:
		return fmt.Sprintf(": it holds both %q and %q", typed, onDisk)
	case heldOnDisk:
		return fmt.Sprintf(": it holds %q%s", onDisk, passClause("; pass", m.corrected))
	case heldTyped:
		return fmt.Sprintf(": it holds %q; rename %q back to %q%s", typed, onDisk, typed, m.recordRenameClause(typed, onDisk))
	default:
		return fmt.Sprintf(": it holds neither %q nor %q", typed, onDisk)
	}
}

// passClause names the corrected path as an argument to paste, or nothing
// when that path cannot be shown on a terminal as it is.
func passClause(lead, corrected string) string {
	if ValidatePath(corrected) != nil || !shellword.Printable(corrected) {
		return ""
	}
	return lead + " --project-config " + shellword.Word(corrected)
}

// recordRenameClause is the other way out when the revision holds the spelling the user typed: record the
// rename on disk with git mv, commit it, and pass the spelling on disk. Only a file can be renamed that
// way, and only when the command can be shown.
func (m caseMismatchError) recordRenameClause(held, onDisk string) string {
	command, ok := gitMoveCommand(held, onDisk)
	if !m.isFile || !ok {
		return ""
	}
	return ", or record the rename with " + command + ", commit it," + passClause(" and pass", m.corrected)
}
