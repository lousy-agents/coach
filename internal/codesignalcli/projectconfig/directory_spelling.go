package projectconfig

import (
	"io"
	"strings"
)

const entryListingBatch = 256

// spellingIn is the entry of directory spelled exactly as name, or else the
// first entry that differs from it only by case, or else name itself.
func spellingIn(root diskRoot, directory, name string) (string, error) {
	listing, err := openDirectory(root, directory)
	if err != nil {
		return "", err
	}
	defer func() { _ = listing.Close() }()
	caseVariant := name
	for {
		names, err := listing.Readdirnames(entryListingBatch)
		exact, variant := firstCaseVariant(names, name)
		if exact {
			return name, nil
		}
		if caseVariant == name {
			caseVariant = variant
		}
		if err == io.EOF {
			return caseVariant, nil
		}
		if err != nil {
			return "", err
		}
	}
}

// firstCaseVariant reports whether names holds name exactly and, if it does
// not, the first entry that differs from name only by case, or name itself.
func firstCaseVariant(names []string, name string) (exact bool, variant string) {
	variant = name
	for _, entry := range names {
		if entry == name {
			return true, name
		}
		if variant == name && strings.EqualFold(entry, name) {
			variant = entry
		}
	}
	return false, variant
}
