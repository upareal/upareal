// story: e01s01 e04s01

// Package db opens the SQLite database with the settings every module relies on.
package db

import (
	"database/sql"
	"fmt"
	"net/url"

	_ "modernc.org/sqlite" // registers the "sqlite" driver (pure Go, no cgo)
)

// Open opens (creating if needed) the SQLite file at path and verifies the
// connection. Every pooled connection gets WAL (required by Litestream),
// foreign keys on, and a 5 s busy timeout.
//
//	conn, err := db.Open("/data/upareal.db")
func Open(path string) (*sql.DB, error) {
	q := url.Values{}
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "foreign_keys(1)")
	// Write transactions take the lock at BEGIN, so concurrent read-then-write
	// transactions wait instead of racing (e.g. two first sign-ins seeding defaults).
	q.Add("_txlock", "immediate")
	dsn := "file:" + path + "?" + q.Encode()

	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("db: open %q: %w", path, err)
	}
	if err := conn.Ping(); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("db: connect to %q (does the directory exist and is it writable?): %w", path, err)
	}
	return conn, nil
}
