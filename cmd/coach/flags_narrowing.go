package main

import (
	"strconv"
	"strings"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func validMinSeverityFlag(f codesignalFlags) bool {
	_, ok := codesignal.ParseSeverityFloor(f.minSeverity)
	return ok || !f.minSeveritySet
}

func severityFloorNames() []string {
	floors := codesignal.SeverityFloors()
	names := make([]string, len(floors))
	for i, floor := range floors {
		names[i] = string(floor)
	}
	return names
}

func severityFloorsUsage() string {
	return strings.Join(severityFloorNames(), "|")
}

func severityFloorsHelp() string {
	return joinOr(severityFloorNames())
}

func severityFloorsWant() string {
	names := severityFloorNames()
	for i, name := range names {
		names[i] = strconv.Quote(name)
	}
	return joinOr(names)
}

func joinOr(words []string) string {
	last := len(words) - 1
	return strings.Join(words[:last], ", ") + ", or " + words[last]
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
