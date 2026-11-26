package evalharness

// faults is one injected rejection: a validation step that rejects the given seeds, in every cell or in one.
type faults struct {
	step    string
	seeds   map[int64]bool
	cell    int
	allCell bool
}

func (f faults) rejectsAt(step string, seed int64, cell int) bool {
	return f.step == step && f.seeds[seed] && (f.allCell || f.cell == cell)
}
