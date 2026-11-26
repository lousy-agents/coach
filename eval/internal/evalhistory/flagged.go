package evalhistory

// Signal is the part of a Coach finding the history needs. StartRow and
// EndRow are Coach's 0-based rows; a file-level finding has no subject and
// a zero location.
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
	Level    Level
	Units    []int
	Unplaced int
}

// FlaggedUnits places each of ruleID's findings in the unit holding its
// first line. The rule's level is its first finding's level.
func FlaggedUnits(inventory Inventory, signals []Signal, ruleID string) Flagged {
	flagged := Flagged{Level: FunctionLevel}
	seen := map[int]bool{}
	first := true
	for _, signal := range signals {
		if signal.RuleID != ruleID {
			continue
		}
		if first {
			flagged.Level, first = signalLevel(signal), false
		}
		index, ok := placeSignal(inventory, signal, flagged.Level)
		if !ok {
			flagged.Unplaced++
			continue
		}
		if !seen[index] {
			seen[index] = true
			flagged.Units = append(flagged.Units, index)
		}
	}
	return flagged
}

func signalLevel(signal Signal) Level {
	if signal.Subject == "" && signal.StartRow == 0 && signal.EndRow == 0 {
		return FileLevel
	}
	return FunctionLevel
}

func placeSignal(inventory Inventory, signal Signal, level Level) (int, bool) {
	if signalLevel(signal) != level {
		return 0, false
	}
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
