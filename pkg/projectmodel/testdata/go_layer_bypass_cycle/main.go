package layerbypasscycle

import (
	"database/sql"
	"net/http"

	"example.com/layerbypasscycle/service"
)

// Handler reaches the pinned database sink directly through queryDB. cycleA
// and cycleB form a call cycle reachable from Handler but never leading to
// the sink, proving the bypass BFS terminates and finds the correct witness
// even when the graph contains an unrelated cycle.
func Handler(w http.ResponseWriter, r *http.Request) {
	cycleA()
	queryDB()
	service.Unused()
}

func cycleA() { cycleB() }

func cycleB() { cycleA() }

func queryDB() {
	var db *sql.DB
	db.Query("SELECT 1")
}
