package evalhistory

import (
	"regexp"
	"strconv"
	"strings"
)

var hunkHeader = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+`)

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
