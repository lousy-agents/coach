package reachabilityboundmethod

import (
	"database/sql"
	"net/http"
)

// Handler is a possible-call-reachability source: its signature is
// identical to net/http.HandlerFunc's func(http.ResponseWriter,
// *http.Request). Its only route to Service.Load is through a synthetic
// bound-method-value wrapper (f := svc.Load; f()), not a direct call.
func Handler(w http.ResponseWriter, r *http.Request) {
	svc := Service{}
	f := svc.Load
	f()
}

// Service's Load method is Handler's real target, reachable only through
// the synthetic bound-method wrapper the SSA builder generates for svc.Load
// above -- Handler itself has no other route to Load or queryDB.
type Service struct{}

func (s Service) Load() {
	queryDB()
}

// queryDB calls a pinned database-access sink through *sql.DB.
func queryDB() {
	var db *sql.DB
	db.Query("SELECT 1")
}
