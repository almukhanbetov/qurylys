// Package seed loads the demo catalogue. All rows it writes have is_demo = TRUE
// and demo company names, so the UI can label them.
package seed

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
)

//go:embed demo.sql
var demoSQL string

// LoadDemo runs demo.sql in one transaction. The script only uses upserts keyed
// by slugs and (product, supplier), so it is safe to run repeatedly.
func LoadDemo(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after commit

	// No arguments: pgx uses the simple protocol, which allows several statements.
	if _, err := tx.ExecContext(ctx, demoSQL); err != nil {
		return fmt.Errorf("apply demo.sql: %w", err)
	}
	return tx.Commit()
}
