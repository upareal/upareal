// story: e05s01

// Package budget owns intent: categories in one level of groups and the amount
// allowed or expected per category per month. It knows nothing about real
// money (ledger) and keeps its own per-user data key, so it can move to its
// own service.
package budget

import (
	"embed"
	"io/fs"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Migrations is the module's schema history, applied with db.Migrate(ctx, conn, "budget", budget.Migrations).
var Migrations = mustSub(migrationFiles, "migrations")

func mustSub(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(err) // dir is embedded at compile time; failure is a build bug
	}
	return sub
}
