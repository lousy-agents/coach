package evalgen

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
)

// Weight is how strongly the profile favours cell: the product, over the
// dimensions a profile can observe, of (count+1)/(total+values), so a value
// it never saw keeps a share and every cell stays possible. A dimension it
// did not observe weighs every value alike, and field, direction and trap,
// which a profile leaves unset, take no part. A count outside the
// generator's range, which no cell holds, scales every weight alike.
func (p Profile) Weight(cell Cell) float64 {
	entries := map[string]int{}
	for _, kind := range EntryKinds {
		entries[kind] = p.entry[kind]
	}
	return smoothed(p.layers[cell.Layers], total(p.layers), 3) *
		smoothed(p.packages[cell.Packages], total(p.packages), 3) *
		smoothed(entries[cell.Entry], total(entries), len(EntryKinds)) *
		smoothed(p.depth[cell.Depth], total(p.depth), 4)
}

func smoothed(count, total, values int) float64 {
	return float64(count+1) / float64(total+values)
}

func total[K comparable](counts map[K]int) int {
	sum := 0
	for _, count := range counts {
		sum += count
	}
	return sum
}

// SampleCells draws count distinct cells, each with probability in
// proportion to its weight, without replacement (Efraimidis and Spirakis:
// the count largest keys u^(1/w)), from a source seeded by seed and read in
// cell-index order, so the same profile, count and seed draw the same cells.
// The keys are compared as log(u)/w, which orders them alike: u^(1/w)
// underflows to 0 once a profile's many observations make w small, and the
// draw would then fall to the first cells by index.
func SampleCells(p Profile, count int, seed int64) ([]Cell, error) {
	if count < 1 || count > CellCount {
		return nil, fmt.Errorf("--count %d is outside 1..%d, the cells there are", count, CellCount)
	}
	source := rand.New(rand.NewSource(seed))
	type keyed struct {
		cell Cell
		key  float64
	}
	keys := make([]keyed, CellCount)
	for index := range keys {
		cell, _ := CellFromIndex(index)
		keys[index] = keyed{cell: cell, key: math.Log(source.Float64()) / p.Weight(cell)}
	}
	sort.SliceStable(keys, func(i, j int) bool { return keys[i].key > keys[j].key })
	cells := make([]Cell, count)
	for i := range cells {
		cells[i] = keys[i].cell
	}
	return cells, nil
}
