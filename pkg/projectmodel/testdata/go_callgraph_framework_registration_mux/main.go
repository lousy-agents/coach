package callgraphframeworkmux

import "net/http"

// echoHandler implements http.Handler via ServeHTTP, not a bare func value.
type echoHandler struct{}

func (echoHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {}

// Register calls (*http.ServeMux).Handle, a method-form registration whose
// first argument is the *http.ServeMux receiver, not the handler.
func Register() {
	mux := http.NewServeMux()
	mux.Handle("/a", echoHandler{})
}
