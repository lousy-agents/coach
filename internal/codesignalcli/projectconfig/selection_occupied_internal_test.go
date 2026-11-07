package projectconfig

import (
	"reflect"
	"testing"
)

// The fields of Selection that say what the policy is, not what the disk holds at the default policy's path.
var selectionFieldsThatAreNotTheDisk = map[string]bool{"Path": true, "Deprecated": true, "Fatal": true}

// Every field that describes the disk makes the default policy's path occupied. A new field that describes the disk
// fails here until CanonicalOccupiedOnDisk counts it, so that the migration that asks that question cannot
// offer a move to a path the diagnostics say is taken.
func TestEveryStateTheDiskCanGiveTheDefaultPathOccupiesIt(t *testing.T) {
	selectionType := reflect.TypeOf(Selection{})
	for i := 0; i < selectionType.NumField(); i++ {
		field := selectionType.Field(i)
		if selectionFieldsThatAreNotTheDisk[field.Name] {
			continue
		}
		var selection Selection
		set := reflect.ValueOf(&selection).Elem().Field(i)
		switch field.Type.Kind() {
		case reflect.Bool:
			set.SetBool(true)
		case reflect.Pointer:
			set.Set(reflect.New(field.Type.Elem()))
		default:
			t.Fatalf("Selection.%s is a %s: say whether it describes the disk, and if it does, how it occupies the path", field.Name, field.Type)
		}

		if !selection.CanonicalOccupiedOnDisk() {
			t.Errorf("Selection.%s set: CanonicalOccupiedOnDisk = false; want true", field.Name)
		}
	}
}

func TestAPolicyThatIsSelectedHoldsNothingOnDisk(t *testing.T) {
	selection := Selection{Path: DefaultPath, Deprecated: true}

	if selection.CanonicalOccupiedOnDisk() {
		t.Errorf("CanonicalOccupiedOnDisk = true for %+v; want false, since nothing is on disk", selection)
	}
}
