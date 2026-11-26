package evalgen

import "slices"

// Index is the inverse of CellFromIndex.
func (c Cell) Index() int {
	index := c.Layers - 3
	index = index*3 + c.Packages - 2
	index = index*len(EntryKinds) + slices.Index(EntryKinds, c.Entry)
	index = index*4 + c.Depth - 2
	index = index*len(FieldTypes) + slices.Index(FieldTypes, c.Field)
	index = index*len(Directions) + slices.Index(Directions, c.Direction)
	return index*len(TrapKinds) + slices.Index(TrapKinds, c.Trap)
}
