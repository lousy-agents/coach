package antipatternfixtures

// constructorDoesRealWork is #871's constructor that does real work, which
// Coach has no Go rule for, so it is written not_covered. It is not scaled:
// the profile says only that such constructors occur.
func constructorDoesRealWork() Template {
	return Template{
		ID: "constructor-does-real-work", Name: "Constructor does real work", Status: "not_covered",
		observe: func(p profile) (observation, bool) {
			count := p.Shapes.Constructor.DoesWork
			return observation{measure: "constructors that do work", value: count, from: count, threshold: 1, limit: count,
				basis: "the profile's count; the template is not scaled"}, count > 0
		},
		Bad: func(int) map[string]string {
			return map[string]string{"go.mod": goMod("store"), "store/store.go": constructorBad}
		},
		Good: func(int) map[string]string {
			return map[string]string{"go.mod": goMod("store"), "store/store.go": constructorGood}
		},
	}
}

const constructorBad = `package store

import (
	"database/sql"
	"os"
)

// Store keeps orders in a database.
type Store struct {
	db *sql.DB
}

// NewStore opens the database named by the environment. The constructor reads configuration and opens a connection
// itself, so a test cannot build a Store without a database and a caller cannot choose another one.
func NewStore() *Store {
	db, err := sql.Open("postgres", os.Getenv("ORDERS_DSN"))
	if err != nil {
		panic(err)
	}
	return &Store{db: db}
}

// Count returns the number of stored orders.
func (s *Store) Count() int {
	var n int
	_ = s.db.QueryRow("SELECT count(*) FROM orders").Scan(&n)
	return n
}
`

const constructorGood = `package store

// DB is the part of a database the store needs.
type DB interface {
	CountOrders() int
}

// Store keeps orders in a database.
type Store struct {
	db DB
}

// NewStore builds a store on the database it is given, so the caller decides which one and a test passes a fake.
func NewStore(db DB) *Store {
	return &Store{db: db}
}

// Count returns the number of stored orders.
func (s *Store) Count() int {
	return s.db.CountOrders()
}
`
