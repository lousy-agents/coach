package callgraphsyntheticwrapper

// Greeter has a method that becomes a synthesized bound-method-value
// wrapper when Greet is evaluated as a first-class value rather than
// called directly.
type Greeter struct{}

// Greet is Greeter's real method body: the actual target CallBound reaches
// only through the synthetic wrapper the SSA builder generates below.
func (g Greeter) Greet() {
	Target()
}

// Target has no body of interest; it exists so Greet's own direct call is
// visible independently of whether CallBound's route into Greet resolves.
func Target() {}

// CallBound binds Greeter.Greet as a first-class func value and calls it.
// The SSA builder resolves f() to a synthesized bound-method wrapper
// function (RelString ending in "$bound") whose Pkg is nil, not to Greet
// directly -- CallBound's only static route to Greet is through that
// wrapper.
func CallBound(g Greeter) {
	f := g.Greet
	f()
}
