package layerbypassdual

import (
	"database/sql"
	"net/http"

	"example.com/layerbypassdual/service"
)

// Handler reaches the pinned database sink via two distinct, equal-length
// routes that both bypass the required service layer: Handler -> AlphaQuery
// -> sink and Handler -> BetaQuery -> sink (each 2 hops, neither touching
// service). service.Unused is called too, purely so the required layer's
// prefix matches a real package in this fixture, exactly as
// go_layer_bypass_bypass_only does.
//
// AlphaQuery and BetaQuery are named so their full ssa.Function.RelString(nil)
// identities -- "example.com/layerbypassdual.AlphaQuery" and
// "example.com/layerbypassdual.BetaQuery" -- differ only at the first
// character after the shared "example.com/layerbypassdual." prefix ('A' vs
// 'B'), making AlphaQuery's identity string strictly lexicographically
// smaller. buildCallGraphAdjacency sorts Handler's neighbor list by that
// same identity string, so bfsShortestPaths enqueues AlphaQuery before
// BetaQuery and reaches the sink through AlphaQuery first -- BetaQuery's
// route to the (already-visited) sink is discovered second and discarded.
func Handler(w http.ResponseWriter, r *http.Request) {
	AlphaQuery()
	BetaQuery()
	service.Unused()
}

func AlphaQuery() {
	var db *sql.DB
	db.Query("SELECT 1")
}

func BetaQuery() {
	var db *sql.DB
	db.Query("SELECT 1")
}
