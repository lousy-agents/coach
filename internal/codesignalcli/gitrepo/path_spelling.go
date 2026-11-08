package gitrepo

import (
	"strings"

	"golang.org/x/text/unicode/norm"
)

// SamePath is whether two spellings name the same path. git on macOS precomposes the names it is
// given and the names it lists, so the name asked about and the name listed can differ in bytes
// alone: the same letters, in the other Unicode normalization form. Nothing else is ignored, not
// case and not a letter, so a record for another path is still another path.
func SamePath(a, b string) bool {
	return a == b || norm.NFC.String(a) == norm.NFC.String(b)
}

// IsBelow is whether path lies inside directory, with the same allowance as SamePath.
func IsBelow(path, directory string) bool {
	return strings.HasPrefix(norm.NFC.String(path), norm.NFC.String(directory)+"/")
}
