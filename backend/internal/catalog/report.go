package catalog

import (
	"context"
	"database/sql"
	"fmt"
	"io"
)

// Report prints what stage 3 will affect. It runs in a read-only transaction,
// works before and after migration 00009 and prints no personal data.
func Report(ctx context.Context, db *sql.DB, w io.Writer) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var version, withGTIN int64
	var hasGTIN bool
	if err := tx.QueryRowContext(ctx, `
		SELECT (SELECT COALESCE(max(version_id), 0) FROM goose_db_version WHERE is_applied),
			EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'products' AND column_name = 'gtin')`).
		Scan(&version, &hasGTIN); err != nil {
		return err
	}
	if version < 8 {
		return fmt.Errorf("schema version %d is too old for the report, need 8", version)
	}
	// Since migration 00010 real offers keep the owner's choice in owner_state.
	shown, offerState := "o.is_active", "CASE WHEN o.is_active THEN ', active' ELSE ', inactive' END"
	if version >= 10 {
		shown = "o.owner_state = 'published'"
		offerState = "CASE WHEN o.is_demo THEN CASE WHEN o.is_active THEN ', active' ELSE ', inactive' END ELSE ', ' || o.owner_state END"
	}
	noGTIN := "TRUE"
	if hasGTIN {
		noGTIN = "p.gtin IS NULL"
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM products WHERE gtin IS NOT NULL`).Scan(&withGTIN); err != nil {
			return err
		}
	}
	fmt.Fprintf(w, "schema version: %d\nproducts with a confirmed GTIN: %d\n", version, withGTIN)

	counts := func(title, query string) error {
		rows, err := tx.QueryContext(ctx, query)
		if err != nil {
			return err
		}
		defer rows.Close()
		fmt.Fprintf(w, "\n%s\n", title)
		n := 0
		for rows.Next() {
			var label string
			var total int64
			if err := rows.Scan(&label, &total); err != nil {
				return err
			}
			fmt.Fprintf(w, "  %-60s %d\n", label, total)
			n++
		}
		if n == 0 {
			fmt.Fprintln(w, "  none")
		}
		return rows.Err()
	}
	kind := `CASE WHEN %s.is_demo THEN 'demo' ELSE 'real' END`
	for _, q := range []struct{ title, query string }{
		{"Products", `SELECT ` + fmt.Sprintf(kind, "p") + ` || CASE WHEN p.is_active THEN ', active' ELSE ', inactive' END, count(*) FROM products p GROUP BY 1 ORDER BY 1`},
		{"Products without a confirmed GTIN", `SELECT ` + fmt.Sprintf(kind, "p") + `, count(*) FROM products p WHERE ` + noGTIN + ` GROUP BY 1 ORDER BY 1`},
		{"Offers", `SELECT ` + fmt.Sprintf(kind, "o") + ` || ` + offerState + `, count(*) FROM offers o GROUP BY 1 ORDER BY 1`},
		{"Active or published real offers that stage 3 rules would hide, by reason", `
			SELECT reason, count(*) FROM offers o
			JOIN products p ON p.id = o.product_id JOIN companies c ON c.id = o.supplier_company_id
			CROSS JOIN LATERAL (VALUES
				('company is not verified', c.verified_request_id IS NULL),
				('company is not a supplier', NOT c.is_supplier),
				('company is suspended', c.status <> 'active'),
				('product has no confirmed GTIN', ` + noGTIN + `),
				('product is a demo card', p.is_demo)) AS r (reason, hit)
			WHERE NOT o.is_demo AND ` + shown + ` AND hit GROUP BY 1 ORDER BY 1`},
		{"Active or published real offers hidden for any reason (product id / company id)", `
			SELECT 'product ' || p.id || ' / company ' || c.id, count(*) FROM offers o
			JOIN products p ON p.id = o.product_id JOIN companies c ON c.id = o.supplier_company_id
			WHERE NOT o.is_demo AND ` + shown + ` AND (c.verified_request_id IS NULL OR NOT c.is_supplier OR c.status <> 'active' OR p.is_demo OR ` + noGTIN + `)
			GROUP BY p.id, c.id ORDER BY p.id, c.id LIMIT 500`},
		{"Demo flag mismatches (offer or product id)", `
			SELECT 'offer ' || o.id || ': demo flag differs from its company or product', 1 FROM offers o
			JOIN products p ON p.id = o.product_id JOIN companies c ON c.id = o.supplier_company_id
			WHERE o.is_demo <> c.is_demo OR (NOT o.is_demo AND p.is_demo) ORDER BY o.id LIMIT 500`},
		{"Possible duplicate real products (same Russian name)", `
			SELECT string_agg(id::text, ', ' ORDER BY id), count(*) FROM products WHERE NOT is_demo
			GROUP BY lower(btrim(name_ru)) HAVING count(*) > 1 ORDER BY min(id) LIMIT 500`},
	} {
		if err := counts(q.title, q.query); err != nil {
			return fmt.Errorf("%s: %w", q.title, err)
		}
	}
	return nil
}
