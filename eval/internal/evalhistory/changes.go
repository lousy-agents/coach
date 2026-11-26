package evalhistory

import (
	"regexp"
	"strconv"
	"strings"
)

var hunkHeader = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+`)

// changedOldLines returns, per path in fix's parent, the lines fix changed
// or deleted. Whitespace-only changes and pure additions change no old line.
func changedOldLines(repoDir, fix string) (map[string][]lineRange, error) {
	diff, err := git(repoDir, "diff", "-w", "-U0", "--no-color", "--no-ext-diff", fix+"^", fix)
	if err != nil {
		return nil, err
	}
	changed := map[string][]lineRange{}
	path := ""
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "--- ") {
			path = oldPath(line)
			continue
		}
		if r, ok := hunkOldRange(line); ok && path != "" {
			changed[path] = append(changed[path], r)
		}
	}
	return changed, nil
}

// oldPath reads the parent's path from a "--- a/<path>" header. An added
// file's "--- /dev/null" has only zero-length hunks, which change no line.
func oldPath(header string) string {
	return strings.TrimPrefix(strings.TrimPrefix(header, "--- "), "a/")
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
