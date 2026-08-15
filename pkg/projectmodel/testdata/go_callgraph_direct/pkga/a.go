package pkga

import "example.com/callgraphdirect/pkgb"

// Caller calls pkgb.Callee directly: a resolvable static call between two
// packages within the same module.
func Caller() {
	pkgb.Callee()
}
