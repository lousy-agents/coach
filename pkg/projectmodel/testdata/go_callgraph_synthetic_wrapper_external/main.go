package callgraphsyntheticwrapperexternal

import "sync"

// CallExternalBound binds (*sync.WaitGroup).Done -- a method on a type
// outside the snapshot, never walked by sortedLocalFunctions regardless of
// whether it is reached through a wrapper -- as a first-class func value
// and calls it. The SSA builder resolves f() to a synthesized bound-method
// wrapper function (RelString ending in "$bound") whose Pkg is nil, unlike
// CallBound in the sibling go_callgraph_synthetic_wrapper fixture, whose
// wrapper's real target (Greeter.Greet) is local to the snapshot.
func CallExternalBound(wg *sync.WaitGroup) {
	f := wg.Done
	f()
}
