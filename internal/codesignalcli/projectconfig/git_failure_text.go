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
