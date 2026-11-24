package projectconfig

// wrongCaseOfCandidate is the case refusal for a discovered candidate the disk lists only in another case. Discovery
// reads the policy only at the canonical path, so a root project.json in another case is no policy to refuse. A path
// the user named is not checked here: ExplicitOnDiskStatus refuses it before anything is selected.
func wrongCaseOfCandidate(rootDir, candidate, namedPath string, tracked func(spelling string) bool) error {
	if namedPath != "" || candidate != CanonicalPath {
		return nil
	}
	if refusal := wrongCaseUnder(rootDir, candidate, chosenByCoach, tracked); refusal != nil {
		return refusal
	}
	return nil
}

func onDiskCandidates(namedPath string) []string {
	if namedPath != "" {
		return []string{namedPath}
	}
	return []string{CanonicalPath, DefaultPath}
}
