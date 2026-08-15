package callgraphframeworkhandle

import "net/http"

// echoHandler implements http.Handler via ServeHTTP, not a bare func value.
type echoHandler struct{}

func (echoHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {}

// Register passes an http.Handler value (not a func) to http.Handle: the
// parameter type is the Handler interface, so it never matches a
// *types.Signature check on its own.
func Register() {
	http.Handle("/a", echoHandler{})
}
