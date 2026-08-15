package callgraphfuncvalue

// CallFunctionValue invokes f, a func-typed parameter: the value bound to f
// at any given call site is not known statically.
func CallFunctionValue(f func()) {
	f()
}
