package evalgen

import (
	"fmt"
	"slices"
	"strings"
)

// Validate names the first dimension that is outside the cell space.
func (c Cell) Validate() error {
	checks := []error{
		inRange("layers", c.Layers, 3, 5),
		inRange("packages", c.Packages, 2, 4),
		oneOf("entry", c.Entry, EntryKinds),
		inRange("depth", c.Depth, 2, 5),
		oneOf("field", c.Field, FieldTypes),
		oneOf("direction", c.Direction, Directions),
		oneOf("trap", c.Trap, TrapKinds),
	}
	for _, err := range checks {
		if err != nil {
			return err
		}
	}
	return nil
}

func inRange(name string, value, low, high int) error {
	if value < low || value > high {
		return fmt.Errorf("%s must be from %d to %d, got %d", name, low, high, value)
	}
	return nil
}

func oneOf(name, value string, allowed []string) error {
	if !slices.Contains(allowed, value) {
		return fmt.Errorf("%s must be one of %s, got %q", name, strings.Join(allowed, ", "), value)
	}
	return nil
}
