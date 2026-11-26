package evalhistory

import (
	"fmt"
	"strings"
)

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
