package evalharness

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
)

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
