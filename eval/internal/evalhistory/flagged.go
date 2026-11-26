package evalhistory

import (
	"fmt"
	"strings"
)

// Signal is the part of a Coach finding the history needs. StartRow and
// EndRow are Coach's 0-based rows; a metric or an import finding may carry
// no location.
type Signal struct {
	RuleID   string
	Path     string
	Subject  string
	StartRow int
	EndRow   int
}

// Level says whether a rule's findings name functions or whole files, and
// so which units its controls are drawn from.
type Level string

const (
	FunctionLevel Level = "function"
	FileLevel     Level = "file"
)

// Flagged is the units one rule's findings fall in, as indexes into the
// inventory's Functions or Files, and how many findings fell in none.
type Flagged struct {
	Level           Level
	Units           []int
	Unplaced        int
	UnplacedReasons []string
}

// ruleLevels is the closed table of rules the history places, and the level
// each is placed at. A rule's level is its kind: a metric or an import edge
// names no function, so its findings are placed in their file whatever rows
// they carry.
var ruleLevels = map[string]Level{
	"complexity.cognitive_complexity": FunctionLevel,
	"complexity.max_nesting_depth":    FileLevel,
	"complexity.branch_density":       FileLevel,
	"architecture.layer_violation":    FileLevel,
	"coupling.deep_relative_import":   FileLevel,
}

// ruleLevel returns the level ruleID's findings are placed at. A rule the
// table does not hold is refused by name, because its level would be a guess.
func ruleLevel(ruleID string) (Level, error) {
	level, ok := ruleLevels[ruleID]
	if !ok {
		return "", fmt.Errorf("rule %s has no placement level in the history's table; add it there before its findings can be placed", ruleID)
	}
	return level, nil
}

// FlaggedUnits places each of ruleID's findings at the rule's level: a
// function finding in the function holding its first line, and a file
// finding in its file.
func FlaggedUnits(inventory Inventory, signals []Signal, ruleID string) (Flagged, error) {
	level, err := ruleLevel(ruleID)
	if err != nil {
		return Flagged{}, err
	}
	flagged := Flagged{Level: level, UnplacedReasons: []string{}}
	seen := map[int]bool{}
	for _, signal := range signals {
		if signal.RuleID != ruleID {
			continue
		}
		index, ok := placeSignal(inventory, signal, level)
		if !ok {
			flagged.Unplaced++
			flagged.UnplacedReasons = append(flagged.UnplacedReasons, unplacedReason(signal))
			continue
		}
		if !seen[index] {
			seen[index] = true
			flagged.Units = append(flagged.Units, index)
		}
	}
	return flagged, nil
}

// unplacedReason names why a finding falls in no unit of the inventory: a file in another language is not inventoried
// at all, and a Go line that no unit holds is a line the inventory does not cover.
func unplacedReason(signal Signal) string {
	if !strings.HasSuffix(signal.Path, ".go") {
		return "language not in the Go history inventory"
	}
	return "no unit of the Go history inventory holds its line"
}

func placeSignal(inventory Inventory, signal Signal, level Level) (int, bool) {
	units, line := inventory.Functions, signal.StartRow+1
	if level == FileLevel {
		units, line = inventory.Files, 1
	}
	for index, unit := range units {
		if unit.Path == signal.Path && unit.Start <= line && line <= unit.End {
			return index, true
		}
	}
	return 0, false
}
