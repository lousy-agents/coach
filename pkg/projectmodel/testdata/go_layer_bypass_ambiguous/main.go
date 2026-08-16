package layerbypassambiguous

import (
	"database/sql"
	"net/http"
)

// Handler has a direct call path to the pinned database sink; this fixture
// exists to prove that a required-layer configuration matching no package
// anywhere in the snapshot suppresses evaluation entirely instead of
// silently reporting the unrelated structural path as a bypass witness.
func Handler(w http.ResponseWriter, r *http.Request) {
	queryDB()
}

func queryDB() {
	var db *sql.DB
	db.Query("SELECT 1")
}
