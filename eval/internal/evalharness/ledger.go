package evalharness

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
)

// LedgerLine is one validated line of the run ledger. Run is the full run record, which is
// self-contained: the trend never reads a local run.json.
type LedgerLine struct {
	Kind     string
	Recorded string
	Cell     string
	Run      map[string]any
}

// LoadLedger reads the JSON Lines ledger at path and validates every non-blank line.
func LoadLedger(path string) ([]LedgerLine, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("ledger not found: %s", path)
	}
	if err != nil {
		return nil, fmt.Errorf("read ledger %s: %w", path, err)
	}
	var lines []LedgerLine
	for index, text := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(text) == "" {
			continue
		}
		line, err := parseLedgerLine(text)
		if err != nil {
			return nil, fmt.Errorf("ledger %s line %d: %w", path, index+1, err)
		}
		lines = append(lines, line)
	}
	return lines, nil
}
