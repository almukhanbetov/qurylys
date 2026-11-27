package categories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"github.com/qurylys/qurylys/backend/internal/audit"
	"github.com/qurylys/qurylys/backend/internal/auth"
	"github.com/qurylys/qurylys/backend/internal/database"
	"github.com/qurylys/qurylys/backend/internal/httpx"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

// ListActive returns all active categories in display order with direct product counts.
func (r *Repository) ListActive(ctx context.Context) ([]Category, error) {
	const q = `
		SELECT c.id, c.parent_id, c.slug, c.name_kk, c.name_ru, c.sort_order,
		       COUNT(p.id) AS direct_product_count
		FROM categories c
		LEFT JOIN products p ON p.category_id = c.id AND p.is_active AND p.blocked_at IS NULL
		WHERE c.is_active
		GROUP BY c.id
		ORDER BY c.sort_order, c.id`

	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("query categories: %w", err)
	}
	defer rows.Close()

	var out []Category
	for rows.Next() {
		var c Category
		var parent sql.NullInt64
		if err := rows.Scan(&c.ID, &parent, &c.Slug, &c.NameKK, &c.NameRU, &c.SortOrder, &c.DirectProductCount); err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}
		if parent.Valid {
			p := parent.Int64
			c.ParentID = &p
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// treeLock serialises changes of the tree, so two concurrent moves cannot form a
// cycle that neither transaction sees (the trigger alone reads old data).
const treeLock = `SELECT pg_advisory_xact_lock(73400011)`

// manager re-checks the permission inside the transaction: a revocation takes
// effect even for a request already in progress.
func manager(ctx context.Context, tx *sql.Tx, p auth.Principal) error {
	if _, err := tx.ExecContext(ctx, treeLock); err != nil {
		return err
	}
	ok, err := auth.HasPermission(ctx, tx, p.UserID, auth.PermCatalogCategories)
	if err != nil {
		return err
	}
	if !ok {
		return httpx.NewError(http.StatusForbidden, "forbidden", "Not allowed")
	}
	return nil
}

func (r *Repository) ListAll(ctx context.Context) ([]AdminCategory, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT c.id, c.parent_id, c.slug, c.name_kk, c.name_ru, c.sort_order, c.is_active, c.version,
			(SELECT count(*) FROM products p WHERE p.category_id = c.id),
			(SELECT count(*) FROM categories ch WHERE ch.parent_id = c.id),
			(SELECT count(*) FROM catalog_requests q WHERE q.category_id = c.id)
		FROM categories c ORDER BY c.sort_order, c.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AdminCategory{}
	for rows.Next() {
		var c AdminCategory
		if err := rows.Scan(&c.ID, &c.ParentID, &c.Slug, &c.NameKK, &c.NameRU, &c.SortOrder, &c.IsActive, &c.Version,
			&c.Products, &c.Children, &c.Requests); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// checkParent: the parent exists and is neither the category nor its descendant.
func checkParent(ctx context.Context, tx *sql.Tx, id int64, parent *int64) error {
	if parent == nil {
		return nil
	}
	var exists, cycle bool
	if err := tx.QueryRowContext(ctx, `
		WITH RECURSIVE up AS (
			SELECT id, parent_id FROM categories WHERE id = $1
			UNION ALL
			SELECT c.id, c.parent_id FROM categories c JOIN up ON c.id = up.parent_id
		)
		SELECT EXISTS (SELECT 1 FROM up), EXISTS (SELECT 1 FROM up WHERE id = $2)`, *parent, id).Scan(&exists, &cycle); err != nil {
		return err
	}
	if !exists {
		return errParent
	}
	if cycle {
		return errCycle
	}
	return nil
}

func (r *Repository) Create(ctx context.Context, p auth.Principal, in Input) (id int64, err error) {
	err = database.WithTx(ctx, r.db, func(tx *sql.Tx) error {
		if err := manager(ctx, tx, p); err != nil {
			return err
		}
		if err := checkParent(ctx, tx, 0, in.ParentID); err != nil {
			return err
		}
		err := tx.QueryRowContext(ctx, `
			INSERT INTO categories (parent_id, slug, name_kk, name_ru, sort_order) VALUES ($1, $2, $3, $4, $5) RETURNING id`,
			in.ParentID, in.Slug, in.NameKK, in.NameRU, in.SortOrder).Scan(&id)
		if database.UniqueViolation(err) == "categories_slug_key" {
			return errSlugTaken
		}
		if err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Event{ActorID: p.UserID, Action: "catalog.category_created",
			Details: map[string]any{"category_id": id, "new": in}})
	})
	return id, err
}

func lockCategory(ctx context.Context, tx *sql.Tx, id int64, version int) (Input, error) {
	var old Input
	var v int
	err := tx.QueryRowContext(ctx, `
		SELECT parent_id, slug, name_kk, name_ru, sort_order, version FROM categories WHERE id = $1 FOR UPDATE`, id).Scan(
		&old.ParentID, &old.Slug, &old.NameKK, &old.NameRU, &old.SortOrder, &v)
	if errors.Is(err, sql.ErrNoRows) {
		return old, errNotFound
	}
	if err != nil {
		return old, err
	}
	if v != version {
		return old, errStale
	}
	return old, nil
}

// Update changes names, order and parent. Card links and ids stay as they are.
func (r *Repository) Update(ctx context.Context, p auth.Principal, id int64, version int, in Input) error {
	return database.WithTx(ctx, r.db, func(tx *sql.Tx) error {
		if err := manager(ctx, tx, p); err != nil {
			return err
		}
		old, err := lockCategory(ctx, tx, id, version)
		if err != nil {
			return err
		}
		if err := checkParent(ctx, tx, id, in.ParentID); err != nil {
			return err
		}
		in.Slug = old.Slug
		sameParent := (in.ParentID == nil) == (old.ParentID == nil) && (in.ParentID == nil || *in.ParentID == *old.ParentID)
		if sameParent && in.NameKK == old.NameKK && in.NameRU == old.NameRU && in.SortOrder == old.SortOrder {
			return nil
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE categories SET parent_id = $2, name_kk = $3, name_ru = $4, sort_order = $5 WHERE id = $1`,
			id, in.ParentID, in.NameKK, in.NameRU, in.SortOrder); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Event{ActorID: p.UserID, Action: "catalog.category_updated",
			Details: map[string]any{"category_id": id, "old": old, "new": in}})
	})
}

// Delete removes an unused category only; the reason is returned otherwise.
func (r *Repository) Delete(ctx context.Context, p auth.Principal, id int64, version int) error {
	return database.WithTx(ctx, r.db, func(tx *sql.Tx) error {
		if err := manager(ctx, tx, p); err != nil {
			return err
		}
		old, err := lockCategory(ctx, tx, id, version)
		if err != nil {
			return err
		}
		var children, cards, requests bool
		if err := tx.QueryRowContext(ctx, `
			SELECT EXISTS (SELECT 1 FROM categories WHERE parent_id = $1),
				EXISTS (SELECT 1 FROM products WHERE category_id = $1),
				EXISTS (SELECT 1 FROM catalog_requests WHERE category_id = $1)`, id).Scan(&children, &cards, &requests); err != nil {
			return err
		}
		switch {
		case children:
			return errHasChildren
		case cards:
			return errHasProducts
		case requests:
			return errHasRequests
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM categories WHERE id = $1`, id); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Event{ActorID: p.UserID, Action: "catalog.category_deleted",
			Details: map[string]any{"category_id": id, "old": old}})
	})
}
