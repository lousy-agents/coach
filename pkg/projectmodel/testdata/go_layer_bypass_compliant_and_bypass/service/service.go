package service

import "database/sql"

// LoadUser calls the pinned database-access sink through *sql.DB.
func LoadUser() {
	var db *sql.DB
	db.Query("SELECT 1")
}
