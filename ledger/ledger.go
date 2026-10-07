// story: e04s01

// Package ledger owns accounts, categories and transactions (double entry with
// signed centavos). It refers to users only by ID and keeps its own per-user
// data key, so it can move to its own service.
package ledger

import (
	"embed"
	"io/fs"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Migrations is the module's schema history, applied with db.Migrate(ctx, conn, "ledger", ledger.Migrations).
var Migrations = mustSub(migrationFiles, "migrations")

func mustSub(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(err) // dir is embedded at compile time; failure is a build bug
	}
	return sub
}
