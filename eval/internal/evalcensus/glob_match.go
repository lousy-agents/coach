package evalcensus

import (
	"path"
	"strings"
)

// validGlob reports whether pattern is well formed. ** is a whole segment that crosses segments; every other
// segment is a path.Match pattern, in which * stays within one segment and ? matches one character.
func validGlob(pattern string) error {
	for _, segment := range strings.Split(pattern, "/") {
		if err := validSegment(segment); err != nil {
			return err
		}
	}
	return nil
}

func validSegment(segment string) error {
	if segment == "**" {
		return nil
	}
	_, err := path.Match(segment, "")
	return err
}

// globMatch reports whether pattern matches the repository-relative, slash-separated name.
func globMatch(pattern, name string) bool {
	return matchSegments(strings.Split(pattern, "/"), strings.Split(name, "/"))
}

func matchSegments(pattern, name []string) bool {
	switch {
	case len(pattern) == 0:
		return len(name) == 0
	case pattern[0] == "**":
		return matchAfterSkip(pattern[1:], name)
	case len(name) == 0:
		return false
	}
	matched, err := path.Match(pattern[0], name[0])
	return err == nil && matched && matchSegments(pattern[1:], name[1:])
}

// matchAfterSkip matches rest against name after skipping zero or more of name's leading segments.
func matchAfterSkip(rest, name []string) bool {
	for skip := range len(name) + 1 {
		if matchSegments(rest, name[skip:]) {
			return true
		}
	}
	return false
}
