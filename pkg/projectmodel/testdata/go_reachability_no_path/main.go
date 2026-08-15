package reachabilitynopath

import (
	"database/sql"
	"net/http"
)

// Handler is a possible-call-reachability source that never calls through
// to a sink.
func Handler(w http.ResponseWriter, r *http.Request) {
	helper()
}

func helper() {}

// unrelatedQuery is a pinned database-access sink, but nothing on Handler's
// call path reaches it.
func unrelatedQuery() {
	var db *sql.DB
	db.Query("x")
}
