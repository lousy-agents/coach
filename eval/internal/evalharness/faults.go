package evalharness

// faults maps a validation step to the seeds it rejects regardless of the program.
type faults map[string]map[int64]bool

func (f faults) rejectsAt(step string, seed int64) bool {
	return f[step][seed]
}
