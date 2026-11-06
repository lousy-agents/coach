package projectreadiness

// PolicyRemedy is the one way out of a policy gap that Coach offers.
type PolicyRemedy int

const (
	// RemedyAuthor writes a new policy. It is the zero value because it is what
	// a policy gap offers unless something says otherwise.
	RemedyAuthor PolicyRemedy = iota
	// RemedyRestore brings back a policy that is committed at the analyzed
	// revision and gone from disk.
	RemedyRestore
	// RemedyNone is for a gap that neither can fix: the path cannot be created,
	// the disk already holds something unusable there, or restoring would
	// overwrite something.
	RemedyNone
)

// NextActionForRemedy is the next action kind a remedy names, and false for a
// remedy that names none. It is the only place a remedy becomes an action kind.
func NextActionForRemedy(remedy PolicyRemedy) (string, bool) {
	switch remedy {
	case RemedyAuthor:
		return NextActionAuthorPolicy, true
	case RemedyRestore:
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
