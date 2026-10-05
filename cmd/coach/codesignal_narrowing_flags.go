package main

import (
	"strconv"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func validMinSeverityFlag(f codesignalFlags) bool {
	_, ok := codesignal.ParseSeverityFloor(f.minSeverity)
	return ok || !f.minSeveritySet
}

func validTopFlag(f codesignalFlags) bool {
	_, ok := parseTopCap(f.top)
	return ok || !f.topSet
}

func parseTopCap(value string) (int, bool) {
	n, err := strconv.Atoi(value)
	return n, err == nil && n > 0
}

// narrowOptionsFor reads the narrowing flags after validation: an unset flag
// holds "", which is no floor and no cap.
func narrowOptionsFor(f codesignalFlags) codesignal.NarrowOptions {
	top, _ := parseTopCap(f.top)
	return codesignal.NarrowOptions{MinSeverity: codesignal.Severity(f.minSeverity), Top: top}
}
