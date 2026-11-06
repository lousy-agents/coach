package projectconfig

import (
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"

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
	}
}

func (m caseMismatchError) Error() string {
	return m.config(nil).Message
}

// adviseOnRevision is err, with a case mismatch turned into the refusal that
// says what the analyzed revision holds and what to do about it, and a policy
// missing from disk turned into a refusal when the revision holds its path in
// another case.
func adviseOnRevision(err error, repoPath string, entries revisionEntries) error {
	var mismatch caseMismatchError
	if errors.As(err, &mismatch) {
		return mismatch.config(entries)
	}
	var found *ConfigError
	if entries != nil && errors.As(err, &found) && found.Kind == KindMissing && found.Cause == CauseAbsent {
		if respelled := respelledInRevision(entries, repoPath); respelled != nil {
			return respelled
		}
	}
	return err
}

// config is the refusal for the mismatch. With no way to ask the revision, or
// when asking fails, it states the two spellings and gives no advice, because
// advice built from the user's own argument can point the wrong way.
func (m caseMismatchError) config(entries revisionEntries) *ConfigError {
	reason := fmt.Sprintf("is stored as %q on disk, not %q; this filesystem ignores case, so the name resolves, but the analyzed revision tells the two spellings apart", m.onDisk, m.typed)
	return newUnusableOnDisk(m.repoPath, reason+m.revisionClause(entries))
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
		return fmt.Sprintf(": it holds %q; rename %q back to %q, or commit the rename%s", typed, onDisk, typed, passClause(" and pass", m.corrected))
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
