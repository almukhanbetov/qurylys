package selleroffers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/qurylys/qurylys/backend/internal/audit"
	"github.com/qurylys/qurylys/backend/internal/auth"
	"github.com/qurylys/qurylys/backend/internal/companies"
	"github.com/qurylys/qurylys/backend/internal/database"
	"github.com/qurylys/qurylys/backend/internal/quantity"
)

type Repository struct{ db *sql.DB }

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

// usableCard is a real card that a supplier may offer.
const usableCard = `p.is_active AND cat.is_active AND NOT p.is_demo AND NOT p.needs_review AND p.gtin IS NOT NULL`

// reasonsSQL lists unmet publication conditions. It is the only definition.
const reasonsSQL = `array_to_string(ARRAY[
	CASE WHEN c.status <> 'active' THEN 'company_suspended' END,
	CASE WHEN NOT c.is_supplier THEN 'company_not_supplier' END,
	CASE WHEN c.verified_request_id IS NULL THEN 'company_not_verified' END,
	CASE WHEN NOT (p.is_active AND cat.is_active) THEN 'card_inactive' END,
	CASE WHEN p.is_demo THEN 'card_demo' END,
	CASE WHEN p.needs_review THEN 'card_needs_review' END,
	CASE WHEN p.gtin IS NULL THEN 'card_gtin_unconfirmed' END,
	CASE WHEN o.price_tiyn IS NULL THEN 'price_missing' END,
	CASE WHEN o.seller_sku IS NULL THEN 'seller_sku_missing' END,
	CASE WHEN o.vat_status IS NULL THEN 'vat_missing' END], ',')`

const offerColumns = `o.id, p.id, p.catalog_no, p.name_kk, p.name_ru, p.unit, COALESCE(p.gtin, ''),
	o.owner_state, COALESCE(o.seller_sku, ''), COALESCE(o.vat_status, ''), COALESCE(trim_scale(o.pack_size)::text, ''),
	trim_scale(o.stock_quantity)::text, COALESCE(o.city, ''),
	o.price_tiyn, o.retail_sale_mode, trim_scale(o.min_order_quantity)::text, trim_scale(o.retail_step)::text,
	o.version, o.updated_at, ` + reasonsSQL

const offerFrom = ` FROM offers o
	JOIN products p ON p.id = o.product_id
	JOIN categories cat ON cat.id = p.category_id
	JOIN companies c ON c.id = o.supplier_company_id
	WHERE NOT o.is_demo AND o.supplier_company_id = $1 `

func (r *Repository) load(ctx context.Context, q database.Querier, companyID int64, offerID *int64) ([]Offer, error) {
	rows, err := q.QueryContext(ctx, `SELECT `+offerColumns+offerFrom+`AND ($2::bigint IS NULL OR o.id = $2) ORDER BY o.id DESC LIMIT 500`, companyID, offerID)
	if err != nil {
		return nil, fmt.Errorf("seller offers: %w", err)
	}
	defer rows.Close()
	out, index := []Offer{}, map[int64]int{}
	for rows.Next() {
		var o Offer
		var reasons string
		if err := rows.Scan(&o.ID, &o.Card.ID, &o.Card.CatalogNo, &o.Card.NameKK, &o.Card.NameRU, &o.Card.Unit, &o.Card.GTIN,
			&o.OwnerState, &o.SellerSKU, &o.VAT, &o.PackSize, &o.Stock, &o.City,
			&o.Retail.PriceTiyn, &o.Retail.SaleMode, &o.Retail.Min, &o.Retail.Step, &o.Version, &o.UpdatedAt, &reasons); err != nil {
			return nil, err
		}
		o.Card.HasOffer, o.Reasons = true, []string{}
		if reasons != "" {
			o.Reasons = strings.Split(reasons, ",")
		}
		index[o.ID] = len(out)
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return out, nil
	}
	tiers, err := q.QueryContext(ctx, `
		SELECT t.offer_id, t.level, t.price_tiyn, t.sale_mode, trim_scale(t.min_quantity)::text, trim_scale(t.step_quantity)::text
		FROM offer_price_tiers t JOIN offers o ON o.id = t.offer_id
		WHERE o.supplier_company_id = $1 AND ($2::bigint IS NULL OR o.id = $2)`, companyID, offerID)
	if err != nil {
		return nil, err
	}
	defer tiers.Close()
	for tiers.Next() {
		var id int64
		var level string
		var v LevelView
		if err := tiers.Scan(&id, &level, &v.PriceTiyn, &v.SaleMode, &v.Min, &v.Step); err != nil {
			return nil, err
		}
		i, ok := index[id]
		switch {
		case !ok: // beyond the list limit
		case level == LevelWholesale:
			out[i].Wholesale = &v
		default:
			out[i].LargeWholesale = &v
		}
	}
	for i := range out {
		for _, l := range []*LevelView{&out[i].Retail, out[i].Wholesale, out[i].LargeWholesale} {
			if l != nil {
				l.calculate(out[i].PackSize, out[i].Stock)
			}
		}
	}
	return out, tiers.Err()
}

func (l *LevelView) calculate(pack, stock string) {
	minQ, _ := quantity.Parse(l.Min)
	stockQ, _ := quantity.Parse(stock)
	l.InStock = stockQ >= minQ
	if l.PriceTiyn == nil {
		return
	}
	if v, ok := quantity.Total(*l.PriceTiyn, minQ); ok {
		l.MinTotalTiyn = &v
	}
	if packQ, ok := quantity.Parse(pack); ok {
		if v, ok := quantity.Total(*l.PriceTiyn, packQ); ok {
			l.PackPriceTiyn = &v
		}
	}
}

func (r *Repository) list(ctx context.Context, p auth.Principal, companyID int64) (ListView, error) {
	role, err := companies.Authorize(ctx, r.db, p, companyID, false, false)
	if err != nil {
		return ListView{}, err
	}
	v := ListView{CanManage: role == companies.RoleOwner}
	var supplier bool
	if err := r.db.QueryRowContext(ctx, `SELECT is_supplier AND status = 'active' FROM companies WHERE id = $1`, companyID).Scan(&supplier); err != nil {
		return v, err
	}
	v.CanCreate = v.CanManage && supplier
	v.Items, err = r.load(ctx, r.db, companyID, nil)
	return v, err
}

func (r *Repository) get(ctx context.Context, p auth.Principal, companyID, id int64) (*View, error) {
	role, err := companies.Authorize(ctx, r.db, p, companyID, false, false)
	if err != nil {
		return nil, err
	}
	o, err := r.one(ctx, r.db, companyID, id)
	if err != nil {
		return nil, err
	}
	return &View{Offer: *o, CanManage: role == companies.RoleOwner}, nil
}

func (r *Repository) one(ctx context.Context, q database.Querier, companyID, id int64) (*Offer, error) {
	list, err := r.load(ctx, q, companyID, &id)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, errNotFound
	}
	return &list[0], nil
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// cards lists approved cards; for the owner choosing one for a new offer.
func (r *Repository) cards(ctx context.Context, p auth.Principal, companyID int64, query string) ([]Card, error) {
	if _, err := companies.Authorize(ctx, r.db, p, companyID, true, false); err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT p.id, p.catalog_no, p.name_kk, p.name_ru, p.unit, p.gtin,
			EXISTS (SELECT 1 FROM offers o WHERE o.product_id = p.id AND o.supplier_company_id = $1)
		FROM products p JOIN categories cat ON cat.id = p.category_id
		WHERE `+usableCard+` AND ($2 = '' OR p.name_kk ILIKE $3 OR p.name_ru ILIKE $3 OR p.brand ILIKE $3
			OR p.catalog_no ILIKE $3 OR p.gtin = lpad($2, 14, '0'))
		ORDER BY p.name_ru, p.id LIMIT 30`, companyID, query, "%"+escapeLike(query)+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Card{}
	for rows.Next() {
		var c Card
		if err := rows.Scan(&c.ID, &c.CatalogNo, &c.NameKK, &c.NameRU, &c.Unit, &c.GTIN, &c.HasOffer); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// lockOwner authorises a change and locks the company. A reviewer (MRX) who is
// not a member gets 403 for any id, as the specification requires (BR-13).
func lockOwner(ctx context.Context, tx *sql.Tx, p auth.Principal, companyID int64, supplierOnly bool) error {
	if _, err := companies.Authorize(ctx, tx, p, companyID, true, true); err != nil {
		if companies.IsNotFound(err) && p.Has(auth.RoleMRX) {
			return errForbidden
		}
		return err
	}
	if !supplierOnly {
		return nil
	}
	var supplier bool
	if err := tx.QueryRowContext(ctx, `SELECT is_supplier FROM companies WHERE id = $1`, companyID).Scan(&supplier); err != nil {
		return err
	}
	if !supplier {
		return errNotSupplier
	}
	return nil
}

func mapUnique(err error) error {
	switch database.UniqueViolation(err) {
	case "offers_product_supplier_key":
		return errOfferExists
	case "offers_seller_sku_key":
		return errSKUTaken
	}
	return err
}

func (r *Repository) create(ctx context.Context, p auth.Principal, companyID, productID int64, in Input) (id int64, err error) {
	err = database.WithTx(ctx, r.db, func(tx *sql.Tx) error {
		if err := lockOwner(ctx, tx, p, companyID, true); err != nil {
			return err
		}
		// The share lock keeps the card and its unit until the offer exists.
		var unit string
		err := tx.QueryRowContext(ctx, `
			SELECT p.unit FROM products p JOIN categories cat ON cat.id = p.category_id
			WHERE p.id = $1 AND `+usableCard+` FOR SHARE OF p`, productID).Scan(&unit)
		if errors.Is(err, sql.ErrNoRows) {
			return errCard
		}
		if err != nil {
			return err
		}
		v, err := in.validate(unit)
		if err != nil {
			return err
		}
		if err := tx.QueryRowContext(ctx, `
			INSERT INTO offers (product_id, supplier_company_id, is_active, owner_state, seller_sku, vat_status, pack_size, stock_quantity, city,
				price_tiyn, retail_sale_mode, min_order_quantity, retail_step)
			VALUES ($1, $2, FALSE, 'draft', NULLIF($3, ''), NULLIF($4, ''), NULLIF($5, '')::numeric, $6::numeric, NULLIF($7, ''), $8, $9, $10::numeric, $11::numeric)
			RETURNING id`,
			productID, companyID, v.SellerSKU, v.VAT, v.PackSize, v.Stock, v.City,
			v.Retail.PriceTiyn, v.Retail.SaleMode, v.Retail.Min, v.Retail.Step).Scan(&id); err != nil {
			return mapUnique(err)
		}
		if err := writeTiers(ctx, tx, id, v); err != nil {
			return err
		}
		return writeEvent(ctx, tx, p, companyID, productID, id, 1, "created", map[string]any{"new": v})
	})
	return id, err
}

func writeTiers(ctx context.Context, tx *sql.Tx, offerID int64, v Input) error {
	for level, l := range map[string]*Level{LevelWholesale: v.Wholesale, LevelLargeWholesale: v.LargeWholesale} {
		if l == nil {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO offer_price_tiers (offer_id, level, price_tiyn, sale_mode, min_quantity, step_quantity)
			VALUES ($1, $2, $3, $4, $5::numeric, $6::numeric)`, offerID, level, l.PriceTiyn, l.SaleMode, l.Min, l.Step); err != nil {
			return fmt.Errorf("price tier: %w", err)
		}
	}
	return nil
}

// writeEvent keeps values in the offer history; audit_events gets no prices.
func writeEvent(ctx context.Context, tx *sql.Tx, p auth.Principal, companyID, productID, offerID int64, version int, action string, details map[string]any) error {
	d, err := json.Marshal(details)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO offer_events (offer_id, company_id, product_id, actor_user_id, action, version, details)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb)`, offerID, companyID, productID, p.UserID, action, version, string(d)); err != nil {
		return fmt.Errorf("offer event: %w", err)
	}
	return audit.Write(ctx, tx, audit.Event{ActorID: p.UserID, CompanyID: companyID, Action: "offer." + action,
		Details: map[string]any{"offer_id": offerID, "version": version}})
}

func (o *Offer) input() Input {
	in := Input{SellerSKU: o.SellerSKU, VAT: o.VAT, PackSize: o.PackSize, Stock: o.Stock, City: o.City, Retail: o.Retail.Level}
	if o.Wholesale != nil {
		in.Wholesale = &o.Wholesale.Level
	}
	if o.LargeWholesale != nil {
		in.LargeWholesale = &o.LargeWholesale.Level
	}
	return in
}

// lockOffer loads the offer under the company lock and checks the version the
// owner was looking at.
func (r *Repository) lockOffer(ctx context.Context, tx *sql.Tx, p auth.Principal, companyID, id int64, version int, supplierOnly bool) (*Offer, error) {
	if err := lockOwner(ctx, tx, p, companyID, supplierOnly); err != nil {
		return nil, err
	}
	o, err := r.one(ctx, tx, companyID, id)
	if err != nil {
		return nil, err
	}
	if o.Version != version {
		return nil, errStale
	}
	return o, nil
}

func (r *Repository) update(ctx context.Context, p auth.Principal, companyID, id int64, version int, in Input) error {
	return database.WithTx(ctx, r.db, func(tx *sql.Tx) error {
		o, err := r.lockOffer(ctx, tx, p, companyID, id, version, true)
		if err != nil {
			return err
		}
		v, err := in.validate(o.Card.Unit)
		if err != nil {
			return err
		}
		// Only a draft may lack a price; a published offer stays complete.
		if (o.OwnerState != StateDraft && v.Retail.PriceTiyn == nil) || (o.OwnerState == StatePublished && (v.SellerSKU == "" || v.VAT == "")) {
			return errIncomplete
		}
		// Old tiers must not be checked against the new pack.
		if _, err := tx.ExecContext(ctx, `DELETE FROM offer_price_tiers WHERE offer_id = $1`, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE offers SET seller_sku = NULLIF($2, ''), vat_status = NULLIF($3, ''), pack_size = NULLIF($4, '')::numeric,
				stock_quantity = $5::numeric, city = NULLIF($6, ''), price_tiyn = $7, retail_sale_mode = $8,
				min_order_quantity = $9::numeric, retail_step = $10::numeric, version = version + 1
			WHERE id = $1`,
			id, v.SellerSKU, v.VAT, v.PackSize, v.Stock, v.City, v.Retail.PriceTiyn, v.Retail.SaleMode, v.Retail.Min, v.Retail.Step); err != nil {
			return mapUnique(err)
		}
		if err := writeTiers(ctx, tx, id, v); err != nil {
			return err
		}
		return writeEvent(ctx, tx, p, companyID, o.Card.ID, id, version+1, "updated", map[string]any{"old": o.input(), "new": v})
	})
}

func (r *Repository) setState(ctx context.Context, p auth.Principal, companyID, id int64, version int, state string) error {
	return database.WithTx(ctx, r.db, func(tx *sql.Tx) error {
		o, err := r.lockOffer(ctx, tx, p, companyID, id, version, state == StatePublished)
		if err != nil {
			return err
		}
		if o.OwnerState == state {
			return nil
		}
		if state == StatePublished && len(o.Reasons) > 0 {
			return errNotPublishable
		}
		if state == StateHidden && o.OwnerState != StatePublished {
			return errStale
		}
		if _, err := tx.ExecContext(ctx, `UPDATE offers SET owner_state = $2, version = version + 1 WHERE id = $1`, id, state); err != nil {
			return err
		}
		return writeEvent(ctx, tx, p, companyID, o.Card.ID, id, version+1, state, map[string]any{"from": o.OwnerState})
	})
}
