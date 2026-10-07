// story: e08s01

// Package planning joins real money (ledger) with intent (budget): it maps
// ledger accounts to budget categories and computes envelopes. It reads the
// other modules only through their Go APIs and stores IDs, never their data.
package planning

import (
	"embed"
	"io/fs"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Migrations is the module's schema history, applied with db.Migrate(ctx, conn, "planning", planning.Migrations).
var Migrations = mustSub(migrationFiles, "migrations")

func mustSub(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(err) // dir is embedded at compile time; failure is a build bug
	}
	return sub
}
