package evalhistory

// unitIdentities maps the identity of every line of every unit at since to
// the unit's index, blaming each file once however many units it holds.
func unitIdentities(repoDir, since string, units []Unit) (map[lineIdentity]int, error) {
	owner := map[lineIdentity]int{}
	blamed := map[string][]lineIdentity{}
	for index, unit := range units {
		all, ok := blamed[unit.Path]
		if !ok {
			var err error
			if all, err = blameFile(repoDir, since, unit.Path); err != nil {
				return nil, err
			}
			blamed[unit.Path] = all
		}
		identities, err := selectLines(all, unit.Path, since, []lineRange{{unit.Start, unit.End}})
		if err != nil {
			return nil, err
		}
		for _, identity := range identities {
			owner[identity] = index
		}
	}
	return owner, nil
}

// touchedUnits returns the indexes of the units whose lines fix changed.
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
		touched = append(touched, ownersOf(identities, owner)...)
	}
	return touched, nil
}

func ownersOf(identities []lineIdentity, owner map[lineIdentity]int) []int {
	var owners []int
	for _, identity := range identities {
		if index, ok := owner[identity]; ok {
			owners = append(owners, index)
		}
	}
	return owners
}
