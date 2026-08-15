package bad

// Bad references an undefined identifier so this package fails to
// type-check.
func Bad() {
	undefinedThing()
}
