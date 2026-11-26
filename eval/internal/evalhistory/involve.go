package evalhistory

// Unit is a span of code at the census SHA: a function or a whole file.
type Unit struct {
	Path       string
	Start, End int
}

// Involvement says, per unit, the first fix that changed one of its lines
// ("" for none), and how many fixes changed no unit's line.
type Involvement struct {
	FixSHA   []string
	Unmapped int
}

// Involve traces each fix's changed lines back by blame identity to the
// units as they stood at since. Fixes are taken in order, so a unit records
// the oldest fix that touched it.
func Involve(repoDir, since string, units []Unit, fixes []FixCommit) (Involvement, error) {
	owner, err := unitIdentities(repoDir, since, units)
	if err != nil {
		return Involvement{}, err
	}
	result := Involvement{FixSHA: make([]string, len(units))}
	for _, fix := range fixes {
		touched, err := touchedUnits(repoDir, fix.SHA, owner)
		if err != nil {
			return Involvement{}, err
		}
		if len(touched) == 0 {
			result.Unmapped++
		}
		for _, index := range touched {
			if result.FixSHA[index] == "" {
				result.FixSHA[index] = fix.SHA
			}
		}
	}
	return result, nil
}

func unitIdentities(repoDir, since string, units []Unit) (map[lineIdentity]int, error) {
	owner := map[lineIdentity]int{}
	for index, unit := range units {
		identities, err := blameLines(repoDir, since, unit.Path, []lineRange{{unit.Start, unit.End}})
		if err != nil {
			return nil, err
		}
		for _, identity := range identities {
			owner[identity] = index
		}
	}
	return owner, nil
}

func touchedUnits(repoDir, fix string, owner map[lineIdentity]int) ([]int, error) {
	changed, err := changedOldLines(repoDir, fix)
	if err != nil {
		return nil, err
	}
	var touched []int
	for path, ranges := range changed {
		identities, err := blameLines(repoDir, fix+"^", path, ranges)
		if err != nil {
			return nil, err
		}
		for _, identity := range identities {
			if index, ok := owner[identity]; ok {
				touched = append(touched, index)
			}
		}
	}
	return touched, nil
}
