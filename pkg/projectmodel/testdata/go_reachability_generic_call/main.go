package reachabilitygenericcall

import (
	"database/sql"
	"net/http"
)

// Handler is a possible-call-reachability source with a resolvable route to
// a pinned sink through two intermediates, mirroring go_reachability_path.
func Handler(w http.ResponseWriter, r *http.Request) {
	loadUser()
}

func loadUser() {
	queryDB()
}

// queryDB calls a pinned database-access sink through *sql.DB.
func queryDB() {
	var db *sql.DB
	db.Query("SELECT 1")
}

// Identity is an unrelated local generic function: no source calls it and
// it has no route to any sink. It exists only to exercise a local generic
// instantiation alongside Handler's resolvable route.
func Identity[T any](v T) T {
	return v
}

// CallGeneric calls Identity through two different instantiations. It has
// no path to Handler or to any sink.
func CallGeneric() {
	Identity(1)
	Identity("a")
}
