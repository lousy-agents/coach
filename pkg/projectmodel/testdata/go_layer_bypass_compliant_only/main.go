package layerbypasscompliant

import (
	"net/http"

	"example.com/layerbypasscompliant/service"
)

// Handler is a possible-call-reachability source that only ever reaches the
// pinned database sink through the required service layer.
func Handler(w http.ResponseWriter, r *http.Request) {
	service.LoadUser()
}
