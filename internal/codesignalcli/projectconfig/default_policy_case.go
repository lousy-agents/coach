package projectconfig

import "strings"

// namesTheDefaultPolicyInAnotherCase is whether the user named the default policy's path in another case. A scan
// without --project-config reads only the default spelling, so a rename to the spelling typed would break it.
func namesTheDefaultPolicyInAnotherCase(repoPath string, chooser pathChooser) bool {
	return chooser == chosenByTheUser && repoPath != CanonicalPath && strings.EqualFold(repoPath, CanonicalPath)
}
