package layerbypassunresolved

import (
	"database/sql"
	"net/http"

	"example.com/layerbypassunresolved/service"
)

// dbAccessor is implemented by more than one type, so a call through a
// dbAccessor value cannot be resolved to a single concrete method
// statically.
type dbAccessor interface {
	Query()
}

type realAccessor struct{}

func (realAccessor) Query() {
	var db *sql.DB
	db.Query("SELECT 1")
}

// Handler's only route to the pinned database sink is through the
// interface-dispatched a.Query() call, which the call graph cannot resolve
// to a CallFact.
func Handler(w http.ResponseWriter, r *http.Request) {
	var a dbAccessor = realAccessor{}
	a.Query()
	service.Unused()
}
