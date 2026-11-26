package antipatternfixtures

import (
	"fmt"
	"path/filepath"
	"strings"
)

// labelDir is the one directory a run may clear: root/label, resolved and
// confirmed to lie strictly inside root, so a label with a separator, "",
// "." and ".." are refused. The label is checked here, not only by the
// profile schema, because clearing a path outside root would delete what
// the run does not own.
func labelDir(root, label string) (string, error) {
	if strings.ContainsAny(label, `/\`) {
		return "", refuse(InputInvalid, fmt.Sprintf("label %q is not one path segment, so its fixtures have no directory of their own under the output root", label))
	}
	base, err := filepath.Abs(root)
	if err != nil {
		return "", refuse(InputInvalid, fmt.Sprintf("the output root cannot be resolved: %v", err))
	}
	dir := filepath.Join(base, label)
	if rel, err := filepath.Rel(base, dir); err != nil || !strictlyInside(rel) {
		return "", refuse(InputInvalid, fmt.Sprintf("label %q does not name a directory strictly inside the output root", label))
	}
	return dir, nil
}

// strictlyInside reports whether rel, the one-segment path of a label
// relative to root, names something below root rather than root itself or
// its parent.
func strictlyInside(rel string) bool {
	return rel != "." && rel != ".."
}
