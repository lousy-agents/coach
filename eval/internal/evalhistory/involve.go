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
		result.record(fix.SHA, touched)
	}
	return result, nil
}

// record notes that fix touched these units, keeping each unit's oldest fix.
func (result *Involvement) record(fix string, touched []int) {
	if len(touched) == 0 {
		result.Unmapped++
	}
	for _, index := range touched {
		if result.FixSHA[index] == "" {
			result.FixSHA[index] = fix
		}
	}
}
