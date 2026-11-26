package evalgen

// TuningCellCount is the number of fixed tuning cells (design §5). They may be tuned on and are never evidence,
// so the held-out panel never names one.
const TuningCellCount = 12

// tuningStride spreads the cells over the index space: layers, the most significant digit, takes each value four
// times, and the odd stride alternates the trap, the least significant.
const tuningStride = 151

// TuningCells returns the twelve tuning cells in a fixed order.
func TuningCells() []Cell {
	cells := make([]Cell, 0, TuningCellCount)
	for index := 0; index < TuningCellCount; index++ {
		cell, _ := CellFromIndex(index * tuningStride)
		cells = append(cells, cell)
	}
	return cells
}

// IsTuning reports whether a cell is one of the twelve.
func IsTuning(cell Cell) bool {
	for _, tuning := range TuningCells() {
		if tuning == cell {
			return true
		}
	}
	return false
}
