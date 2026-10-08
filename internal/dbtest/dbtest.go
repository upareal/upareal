// story: e09s02

// Package dbtest holds database helpers shared by module tests.
package dbtest

import (
	"database/sql"
	"testing"
)

// UserRows counts userID's rows in every table named prefix* that has a
// user_id column, read from the live schema so new tables are never missed.
func UserRows(t testing.TB, conn *sql.DB, prefix, userID string) map[string]int {
	t.Helper()
	rows, err := conn.Query(`SELECT m.name FROM sqlite_master m
		WHERE m.type = 'table' AND substr(m.name, 1, length(?)) = ?
		  AND EXISTS (SELECT 1 FROM pragma_table_info(m.name) WHERE name = 'user_id')`, prefix, prefix)
	if err != nil {
		t.Fatal(err)
	}
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		tables = append(tables, name)
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	if len(tables) == 0 {
		t.Fatalf("no %s* tables with user_id", prefix)
	}
	out := map[string]int{}
	for _, table := range tables {
		var n int
		// table comes from sqlite_master, not user input.
		if err := conn.QueryRow(`SELECT count(*) FROM "`+table+`" WHERE user_id = ?`, userID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		out[table] = n
	}
	return out
}

// AssertGone fails if userID has rows left in any prefix* table.
func AssertGone(t testing.TB, conn *sql.DB, prefix, userID string) {
	t.Helper()
	for table, n := range UserRows(t, conn, prefix, userID) {
		if n != 0 {
			t.Errorf("%s still has %d rows of the deleted user", table, n)
		}
	}
}
