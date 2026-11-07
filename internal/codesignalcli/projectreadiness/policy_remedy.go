package projectreadiness

// PolicyRemedy is the one way out of a policy gap that Coach offers.
type PolicyRemedy int

const (
	// RemedyNone is for a gap that neither can fix: the path cannot be created,
	// the disk already holds something unusable there, or restoring would
	// overwrite something. It is the zero value on purpose, so a check or error
	// that never says which way out it offers offers nothing, as an unset error
	// kind is deliberately non-actionable.
	RemedyNone PolicyRemedy = iota
	// RemedyAuthor writes a new policy. Every site that offers it says so.
	RemedyAuthor
	// RemedyRestore brings back a policy that is committed at the analyzed
	// revision and gone from disk.
	RemedyRestore
	// RemedyRestoreOverEmptyEntry is RemedyRestore for a committed policy whose index
	// holds only an empty entry for it (git add --intent-to-add, or an emptied file
	// staged). It names the same action; it is its own value so the sentence that
	// explains the restore can say why it loses nothing.
	RemedyRestoreOverEmptyEntry
)

// NextActionForRemedy is the next action kind a remedy names, and false for a
// remedy that names none. It is the only place a remedy becomes an action kind.
func NextActionForRemedy(remedy PolicyRemedy) (string, bool) {
	switch remedy {
	case RemedyAuthor:
		return NextActionAuthorPolicy, true
	case RemedyRestore, RemedyRestoreOverEmptyEntry:
		return NextActionRestorePolicy, true
	}
	return "", false
}

// NextActionForGap is the next action readiness names for a gap code once the
// remedy for its policy is known. A gap that is not a policy gap is unaffected
// by the remedy.
func NextActionForGap(code string, remedy PolicyRemedy) (string, bool) {
	if code == GapPolicyMissing || code == GapPolicyInvalid {
		return NextActionForRemedy(remedy)
	}
	return NextActionForGapCode(code)
}
