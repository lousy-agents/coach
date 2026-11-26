package evalharness

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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

func antipatternSetup(args []string, stderr io.Writer) (coach, root string, err error) {
	flags := flag.NewFlagSet("coach-eval antipatterns", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&coach, "coach", "", "the coach binary that scans the fixtures")
	flags.StringVar(&root, "fixtures", filepath.Join("eval", "fixtures", "antipatterns"), "the directory of fixtures")
	if err = flags.Parse(args); err != nil {
		return "", "", err
	}
	if coach == "" {
		return "", "", fmt.Errorf("--coach names the coach binary that scans the fixtures")
	}
	if _, err = os.Stat(coach); err != nil {
		return "", "", fmt.Errorf("the coach binary %s is not usable: %v", coach, err)
	}
	for _, tool := range []string{"git", "go"} {
		if _, err = exec.LookPath(tool); err != nil {
			return "", "", fmt.Errorf("%s is not on PATH: the fixtures are built and committed before they are scanned", tool)
		}
	}
	return coach, root, nil
}

func loadAntipatterns(root string) ([]antipattern, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("the fixtures directory %s cannot be read: %v", root, err)
	}
	var fixtures []antipattern
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		fixture, err := loadAntipattern(filepath.Join(root, entry.Name()))
		if err != nil {
			return nil, err
		}
		fixtures = append(fixtures, fixture)
	}
	if len(fixtures) == 0 {
		return nil, fmt.Errorf("no fixture under %s", root)
	}
	sort.Slice(fixtures, func(i, j int) bool { return fixtures[i].ID < fixtures[j].ID })
	return fixtures, nil
}
