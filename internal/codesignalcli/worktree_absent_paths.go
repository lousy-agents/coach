package codesignalcli

import "slices"

// notOverlaid is every candidate path the overlay did not serve from disk. It
// includes a path that is on disk nowhere and in the committed revision nowhere,
// such as the new path of a rename that was then deleted: the overlay records no
// outcome for it, yet reading it from the committed revision would fail.
func notOverlaid(candidates []string, overlaid map[string]bool) []string {
	return slices.DeleteFunc(slices.Clone(candidates), func(name string) bool { return overlaid[name] })
}
