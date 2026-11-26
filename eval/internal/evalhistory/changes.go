package evalhistory

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var hunkHeader = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+`)

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

// parseNameStatus reads `git diff --name-status -z`. An added file and a
// copy's source change no old line, so neither is listed.
func parseNameStatus(listing string) ([]pathChange, error) {
	fields := strings.Split(strings.TrimSuffix(listing, "\x00"), "\x00")
	var changes []pathChange
	for i := 0; i < len(fields) && fields[i] != ""; {
		status, paths := fields[i], 1
		if status[0] == 'R' || status[0] == 'C' {
			paths = 2
		}
		if i+paths >= len(fields) {
			return nil, fmt.Errorf("unexpected name-status listing at %q", status)
		}
		switch status[0] {
		case 'R':
			changes = append(changes, pathChange{old: fields[i+1], pathspec: fields[i+1 : i+3]})
		case 'A', 'C':
		default:
			changes = append(changes, pathChange{old: fields[i+1], pathspec: fields[i+1 : i+2]})
		}
		i += 1 + paths
	}
	return changes, nil
}

// oldHunks reads the old-side ranges of the hunks in fix's diff limited to
// pathspec, taken literally so a name like "[a].go" matches only itself.
func oldHunks(repoDir, fix string, pathspec []string) ([]lineRange, error) {
	args := []string{"diff", "-w", "-U0", "-M", "--no-color", "--no-ext-diff", fix + "^", fix, "--"}
	for _, path := range pathspec {
		args = append(args, ":(literal)"+path)
	}
	diff, err := git(repoDir, args...)
	if err != nil {
		return nil, err
	}
	var ranges []lineRange
	for _, line := range strings.Split(diff, "\n") {
		if r, ok := hunkOldRange(line); ok {
			ranges = append(ranges, r)
		}
	}
	return ranges, nil
}

func hunkOldRange(line string) (lineRange, bool) {
	match := hunkHeader.FindStringSubmatch(line)
	if match == nil {
		return lineRange{}, false
	}
	start, _ := strconv.Atoi(match[1])
	count := 1
	if match[2] != "" {
		count, _ = strconv.Atoi(match[2])
	}
	if count == 0 {
		return lineRange{}, false
	}
	return lineRange{start: start, end: start + count - 1}, true
}
