package evalgen

import "fmt"

// CellCount is the size of the structural-cell space per language (design §5).
const CellCount = 3 * 3 * 4 * 4 * 3 * 2 * 2

var (
	EntryKinds = []string{"function", "method", "http_handler", "command"}
	FieldTypes = []string{"scalar", "collection", "nested"}
	Directions = []string{"read", "write"}
	TrapKinds  = []string{"layering", "complexity"}
)

// Cell is one point in the structural-cell space. Vocabulary is not a dimension: it varies only the surface.
type Cell struct {
	Layers    int    `json:"layers"`
	Packages  int    `json:"packages"`
	Entry     string `json:"entry"`
	Depth     int    `json:"depth"`
	Field     string `json:"field"`
	Direction string `json:"direction"`
	Trap      string `json:"trap"`
}

// CellFromIndex decodes a cell index; layers is the most significant digit and trap the least.
func CellFromIndex(index int) (Cell, error) {
	if index < 0 || index >= CellCount {
		return Cell{}, fmt.Errorf("cell %d is out of range 0..%d", index, CellCount-1)
	}
	rest := index
	digit := func(base int) int {
		value := rest % base
		rest /= base
		return value
	}
	trap, direction, field := digit(len(TrapKinds)), digit(len(Directions)), digit(len(FieldTypes))
	depth, entry, packages, layers := digit(4), digit(len(EntryKinds)), digit(3), digit(3)
	return Cell{
		Layers: 3 + layers, Packages: 2 + packages, Entry: EntryKinds[entry], Depth: 2 + depth,
		Field: FieldTypes[field], Direction: Directions[direction], Trap: TrapKinds[trap],
	}, nil
}
