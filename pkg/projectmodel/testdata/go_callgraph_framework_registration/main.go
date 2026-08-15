package callgraphframework

import "net/http"

// Handler is registered, not called directly; net/http invokes it later
// through a mechanism invisible to static call resolution.
func Handler(w http.ResponseWriter, r *http.Request) {}

// Register passes Handler as a value to http.HandleFunc.
func Register() {
	http.HandleFunc("/x", Handler)
}
