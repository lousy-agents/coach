package evaldetect

import (
	"fmt"
	"strings"
)

var tsSourceSuffixes = []string{".ts", ".tsx", ".mts", ".cts"}

func isTSSource(name string) bool {
	for _, suffix := range tsSourceSuffixes {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

// refuseJSX refuses a .tsx source: JSX text is outside the subset the lexer
// reads, so lexing it could silently miss an import.
func refuseJSX(rel string) error {
	if strings.HasSuffix(rel, ".tsx") {
		return fmt.Errorf("%w: %s: a .tsx source; JSX is outside the subset the lexer reads", ErrRefused, rel)
	}
	return nil
}
