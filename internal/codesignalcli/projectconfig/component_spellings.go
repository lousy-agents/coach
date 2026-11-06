package projectconfig

import (
	"fmt"
	"path"
	"slices"
	"sort"
	"strings"
)

// resolveComponent is the entry of names spelled exactly as part, else the one
// entry that differs from it only by case, else nothing; with several such
// entries it returns them, sorted, for the caller to put to the user.
func resolveComponent(names []string, part string) (name string, variants []string) {
	if slices.Contains(names, part) {
		return part, nil
	}
	for _, candidate := range names {
		if strings.EqualFold(candidate, part) {
			variants = append(variants, candidate)
		}
	}
	if len(variants) == 1 {
		return variants[0], nil
	}
	sort.Strings(variants)
	return "", variants
}

func joinedBelow(directory string, names []string) []string {
	joined := make([]string, len(names))
	for i, name := range names {
		joined[i] = path.Join(directory, name)
	}
	return joined
}

func quotedList(items []string) string {
	quoted := make([]string, len(items))
	for i, item := range items {
		quoted[i] = fmt.Sprintf("%q", item)
	}
	return strings.Join(quoted, ", ")
}
