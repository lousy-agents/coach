package evalhistory

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// lineIdentity names a line by where it came from: the commit that last
// changed it, and its path and line number in that commit. Two blames that
// agree on it are looking at the same line, however the file moved since.
type lineIdentity struct {
	commit string
	path   string
	line   int
}

// lineRange is an inclusive, 1-based range of lines in one file.
type lineRange struct {
	start, end int
}

var porcelainHeader = regexp.MustCompile(`^([0-9a-f]{40}) (\d+) \d+`)

// blameLines returns the identity of each line in ranges of path at rev,
// ignoring whitespace and following moves and copies (-C implies -M). It blames the whole
// file, because a line range narrows what copy detection can match: a single
// line is shorter than git's minimum block and would stay with the commit
// that moved it.
func blameLines(repoDir, rev, path string, ranges []lineRange) ([]lineIdentity, error) {
	output, err := git(repoDir, "blame", "-w", "-C", "--line-porcelain", rev, "--", path)
	if err != nil {
		return nil, err
	}
	all, err := parsePorcelain(output)
	if err != nil {
		return nil, err
	}
	var selected []lineIdentity
	for _, r := range ranges {
		if r.start < 1 || r.end > len(all) {
			return nil, fmt.Errorf("%s:%d-%d is outside the file's %d lines at %s", path, r.start, r.end, len(all), rev)
		}
		selected = append(selected, all[r.start-1:r.end]...)
	}
	return selected, nil
}

func parsePorcelain(output string) ([]lineIdentity, error) {
	var identities []lineIdentity
	var current lineIdentity
	for _, line := range strings.Split(output, "\n") {
		switch {
		case strings.HasPrefix(line, "\t"):
			identities = append(identities, current)
		case strings.HasPrefix(line, "filename "):
			current.path = strings.TrimPrefix(line, "filename ")
		default:
			if match := porcelainHeader.FindStringSubmatch(line); match != nil {
				number, err := strconv.Atoi(match[2])
				if err != nil {
					return nil, err
				}
				current = lineIdentity{commit: match[1], line: number}
			}
		}
	}
	return identities, nil
}
