package layerbypassboundmethod

import (
	"database/sql"
	"net/http"

	"example.com/layerbypassboundmethod/service"
)

// Handler reaches the pinned database sink only through a synthetic
// bound-method-value wrapper: q.Query is bound off queryProxy as a
// first-class func value (f := q.Query) rather than called directly, so
// the SSA builder resolves f() to a synthesized "$bound" wrapper whose real
// target, queryProxy.Query, never routes through the required service
// layer. service.Unused is called too but is unrelated dead-end code that
// never reaches the sink, present only so the required layer's prefix
// matches a real package in this fixture.
func Handler(w http.ResponseWriter, r *http.Request) {
	q := queryProxy{}
	f := q.Query
	f()
	service.Unused()
}

type queryProxy struct{}

func (queryProxy) Query() {
	var db *sql.DB
	db.Query("SELECT 1")
}
