package reachabilitypath

import (
	"database/sql"
	"net/http"
)

// Handler is a possible-call-reachability source: its signature is
// identical to net/http.HandlerFunc's func(http.ResponseWriter,
// *http.Request).
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
