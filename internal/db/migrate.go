// story: e03s01
package db

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"

	"github.com/pressly/goose/v3"
)

// Migrate applies a module's embedded goose SQL migrations (fsys holds the
// .sql files) and tracks them in its own table goose_<module>, so each module
// can later move to its own service with its schema history.
//
//	err := db.Migrate(ctx, conn, "users", users.Migrations)
func Migrate(ctx context.Context, conn *sql.DB, module string, fsys fs.FS) error {
	p, err := goose.NewProvider(goose.DialectSQLite3, conn, fsys, goose.WithTableName("goose_"+module))
	if err != nil {
		return fmt.Errorf("db: migrations for module %q: %w", module, err)
	}
	if _, err := p.Up(ctx); err != nil {
		return fmt.Errorf("db: migrate module %q: %w", module, err)
	}
	return nil
}
