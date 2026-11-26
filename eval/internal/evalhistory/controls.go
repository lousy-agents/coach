package evalhistory

import (
	"math/bits"
	"math/rand/v2"
)

// Draw is the controls drawn for a rule, as indexes into its unit pool, and
// how many it could not draw because a size bin ran short.
type Draw struct {
	Controls  []int
	Shortfall int
}

// DrawControls draws, for each flagged unit in turn, k unflagged units of
// the same size bin from pool, seeded and without replacement. A unit's
// size bin is the floor of log2 of its line count.
func DrawControls(pool []Unit, flagged []int, k int, seed int64) Draw {
	random := rand.New(rand.NewPCG(uint64(seed), 0))
	used := map[int]bool{}
	for _, index := range flagged {
		used[index] = true
	}
	var draw Draw
	for _, index := range flagged {
		candidates := binCandidates(pool, used, sizeBin(pool[index]))
		random.Shuffle(len(candidates), func(i, j int) { candidates[i], candidates[j] = candidates[j], candidates[i] })
		taken := candidates[:min(k, len(candidates))]
		for _, control := range taken {
			used[control] = true
		}
		draw.Controls = append(draw.Controls, taken...)
		draw.Shortfall += k - len(taken)
	}
	return draw
}

func binCandidates(pool []Unit, used map[int]bool, bin int) []int {
	var candidates []int
	for index, unit := range pool {
		if !used[index] && sizeBin(unit) == bin {
			candidates = append(candidates, index)
		}
	}
	return candidates
}

func sizeBin(unit Unit) int {
	return bits.Len(uint(unit.End-unit.Start+1)) - 1
}
