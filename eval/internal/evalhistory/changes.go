package evalhistory

import "fmt"

// pathChange is one path fix changed: its name in fix's parent, and the
// pathspec that limits a diff to it (both names, for a rename).
type pathChange struct {
	old      string
	pathspec []string
}

// changedOldLines returns, per path in fix's parent, the lines fix changed
// or deleted. Whitespace-only changes and pure additions change no old line.
// Paths come from git's NUL-separated listing and never from a diff header,
// which git quotes, tab-ends, or lets a removed "-- " line imitate.
func changedOldLines(repoDir, fix string) (map[string][]lineRange, error) {
	listing, err := git(repoDir, "diff", "--name-status", "-z", "-M", "--no-color", fix+"^", fix)
	if err != nil {
		return nil, err
	}
	changes, err := parseNameStatus(listing)
	if err != nil {
		return nil, fmt.Errorf("fix %s: %w", fix, err)
	}
	changed := map[string][]lineRange{}
	for _, change := range changes {
		ranges, err := oldHunks(repoDir, fix, change.pathspec)
		if err != nil {
			return nil, err
		}
		if len(ranges) > 0 {
			changed[change.old] = append(changed[change.old], ranges...)
		}
	}
	return changed, nil
}
