package evalharness

import (
	"encoding/json"
	"fmt"
	"io"
)

// Exit codes of `coach-eval antipatterns` (#871).
const (
	AntipatternsOK     = 0
	AntipatternsFailed = 1
	AntipatternsUnused = 2
)

type fixtureReport struct {
	ID     string        `json:"id"`
	Name   string        `json:"name"`
	Source string        `json:"source"`
	Seed   string        `json:"seed"`
	Status string        `json:"status"`
	Bad    variantReport `json:"bad"`
	Good   variantReport `json:"good"`
}

type antipatternSummary struct {
	Pass       int      `json:"pass"`
	Fail       int      `json:"fail"`
	NotCovered []string `json:"not_covered"`
	Incomplete []string `json:"incomplete"`
}

type antipatternTable struct {
	Fixtures []fixtureReport    `json:"fixtures"`
	Summary  antipatternSummary `json:"summary"`
}

// Antipatterns scans the bad and the good variant of every fixture with coach and prints, as JSON, what was expected
// against what coach reported. It exits 1 when a covered fixture fails, and 2 when its input is unusable.
func Antipatterns(args []string, stdout, stderr io.Writer) int {
	coach, root, err := antipatternSetup(args, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "coach-eval antipatterns: %v\n", err)
		return AntipatternsUnused
	}
	fixtures, err := loadAntipatterns(root)
	if err != nil {
		fmt.Fprintf(stderr, "coach-eval antipatterns: %v\n", err)
		return AntipatternsUnused
	}
	table, err := scanAntipatterns(coach, fixtures)
	if err != nil {
		fmt.Fprintf(stderr, "coach-eval antipatterns: %v\n", err)
		return AntipatternsUnused
	}
	for _, line := range table.failures() {
		fmt.Fprintln(stderr, line)
	}
	encoded, _ := json.MarshalIndent(table, "", "  ")
	fmt.Fprintf(stdout, "%s\n", encoded)
	if table.Summary.Fail > 0 {
		return AntipatternsFailed
	}
	return AntipatternsOK
}
