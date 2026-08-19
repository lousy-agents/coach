package callgraphgeneric

// Identity is a local generic function. Every call site below resolves to a
// per-instantiation synthetic wrapper (fn.Pkg == nil), not to Identity
// itself -- but Identity's own outgoing call to Sink must still be walked
// directly, since Identity itself (the generic origin) has a body and a
// declaring package.
func Identity[T any](v T) T {
	Sink()
	return v
}

func Sink() {}

// CallGeneric calls Identity through two different instantiations.
func CallGeneric() {
	Identity(1)
	Identity("a")
}
