package layerbypassmixed

import (
	"database/sql"
	"net/http"

	"example.com/layerbypassmixed/service"
)

// Handler reaches the pinned database sink two ways: once compliantly
// through service (the required layer, 3 hops: Handler, service.LoadUser,
// sink), and once directly through directQuery -> rawQuery, bypassing the
// required layer entirely (4 hops: Handler, directQuery, rawQuery, sink).
// The bypass route is intentionally longer than the compliant route so a
// shortest-path search that fails to remove the required layer still finds
// the compliant route first, keeping the compliant route a real control.
func Handler(w http.ResponseWriter, r *http.Request) {
	service.LoadUser()
	directQuery()
}

func directQuery() {
	rawQuery()
}

func rawQuery() {
	var db *sql.DB
	db.Query("SELECT 1")
}
