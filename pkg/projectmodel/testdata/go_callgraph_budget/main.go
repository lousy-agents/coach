package callgraphbudget

// A calls B, B calls C, C calls D: three resolvable call sites, enough to
// observe a budget cutting the walk short before it reaches all of them.
func A() { B() }
func B() { C() }
func C() { D() }
func D() {}
