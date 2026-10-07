// story: e03s01

// Package users owns sign-in, user records and sessions. Other modules refer to
// users only by ID; web never touches users tables.
package users

import (
	"embed"
	"io/fs"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Migrations is the module's schema history, applied with db.Migrate(ctx, conn, "users", users.Migrations).
var Migrations = mustSub(migrationFiles, "migrations")

func mustSub(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(err) // dir is embedded at compile time; failure is a build bug
	}
	return sub
}
