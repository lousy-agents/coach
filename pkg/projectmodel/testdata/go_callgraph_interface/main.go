package callgraphinterface

// Greeter is implemented by more than one type, so a call through a Greeter
// value cannot be resolved to a single concrete method statically.
type Greeter interface {
	Greet()
}

type realGreeter struct{}

func (realGreeter) Greet() {}

type otherGreeter struct{}

func (otherGreeter) Greet() {}

// CallInterface dispatches through the Greeter interface value g: the
// concrete receiver is unknown until runtime.
func CallInterface(g Greeter) {
	g.Greet()
}
