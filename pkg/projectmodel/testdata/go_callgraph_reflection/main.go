package callgraphreflection

import "reflect"

// Target is invoked only through reflection below, never called directly.
func Target() {}

// CallReflection dispatches Target via reflect.Value.Call: the real target
// is chosen at runtime and invisible to static call resolution.
func CallReflection() {
	v := reflect.ValueOf(Target)
	v.Call(nil)
}
