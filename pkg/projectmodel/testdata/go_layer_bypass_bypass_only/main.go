package layerbypassonly

import (
	"database/sql"
	"net/http"

	"example.com/layerbypassonly/service"
)

// Handler reaches the pinned database sink only directly through queryDB;
// service is called too but is unrelated dead-end code that never reaches
// the sink, present only so the required layer's prefix matches a real
// package in this fixture.
func Handler(w http.ResponseWriter, r *http.Request) {
	queryDB()
	service.Unused()
}

func queryDB() {
	var db *sql.DB
	db.Query("SELECT 1")
}
