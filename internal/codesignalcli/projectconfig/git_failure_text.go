package projectconfig

import "strconv"

const maxGitErrorRunes = 200

// describe words the failed question with git's error, quoted so that no
// control character in git's output reaches the terminal and cut to a bound.
func (f *gitFailure) describe() string {
	text := []rune(f.err.Error())
	if len(text) > maxGitErrorRunes {
		text = append(text[:maxGitErrorRunes], '…')
	}
	return f.question + " (git said " + strconv.Quote(string(text)) + ")"
}

// Error is the failure worded for the user, so a gitFailure can stand as the
// GitFailure of a ConfigError.
func (f *gitFailure) Error() string {
	return "could not check " + f.describe()
}

// Unwrap is git's own error, so a caller that classifies the failure reaches it
// and not only the text above.
func (f *gitFailure) Unwrap() error {
	return f.err
}
