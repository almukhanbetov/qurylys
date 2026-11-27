-- +goose Up
-- Units sold only in whole numbers; the others are measured.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION unit_is_discrete(unit TEXT) RETURNS BOOLEAN AS $$
    SELECT unit IN ('piece', 'bag', 'pack', 'roll', 'sheet', 'pallet')
$$ LANGUAGE sql IMMUTABLE;
-- +goose StatementEnd

-- Quantity rule of one price level, in base units, exact decimals:
--   the minimum is a multiple of the step;
--   'pack':  the step is a whole number of packs;
--   'piece': the step is 1 for discrete units, any exact value for measured ones;
--   discrete units allow whole numbers only.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION sale_rule_ok(unit TEXT, mode TEXT, pack NUMERIC, min_qty NUMERIC, step NUMERIC) RETURNS BOOLEAN AS $$
    SELECT COALESCE(min_qty > 0 AND step > 0 AND min_qty % step = 0
        AND (unit NOT IN ('piece', 'bag', 'pack', 'roll', 'sheet', 'pallet')
            OR (min_qty = trunc(min_qty) AND step = trunc(step) AND (pack IS NULL OR pack = trunc(pack))))
        AND CASE mode
            WHEN 'pack' THEN pack IS NOT NULL AND step % pack = 0
            WHEN 'piece' THEN unit NOT IN ('piece', 'bag', 'pack', 'roll', 'sheet', 'pallet') OR step = 1
            ELSE FALSE END, FALSE)
$$ LANGUAGE sql IMMUTABLE;
-- +goose StatementEnd

-- price_tiyn stays the retail price (РРЦ) and min_order_quantity its minimum.
ALTER TABLE offers
    -- The owner's choice. Public visibility is derived separately (step 3.4).
    ADD COLUMN owner_state       TEXT           NOT NULL DEFAULT 'draft',
    ADD COLUMN seller_sku        TEXT,
    ADD COLUMN vat_status        TEXT,
    ADD COLUMN pack_size         NUMERIC(14, 3),
    ADD COLUMN retail_sale_mode  TEXT           NOT NULL DEFAULT 'piece',
    ADD COLUMN retail_step       NUMERIC(14, 3) NOT NULL DEFAULT 1,
    -- Optimistic lock for prices and stock.
    ADD COLUMN version           INTEGER        NOT NULL DEFAULT 1,
    ALTER COLUMN price_tiyn DROP NOT NULL;

-- Real offers leave the legacy public flag for good: the previous release reads
-- only is_active, so after a rollback it cannot show drafts, hidden offers or
-- offers of unverified companies. The owner's choice is kept in owner_state.
UPDATE offers SET owner_state = CASE WHEN is_active THEN 'hidden' ELSE 'draft' END, is_active = FALSE WHERE NOT is_demo;

ALTER TABLE offers
    ADD CONSTRAINT offers_real_not_legacy_public CHECK (is_demo OR NOT is_active),
    ADD CONSTRAINT offers_owner_state_valid CHECK (owner_state IN ('draft', 'published', 'hidden')),
    ADD CONSTRAINT offers_vat_status_valid CHECK (vat_status IN ('with_vat', 'without_vat')),
    ADD CONSTRAINT offers_seller_sku_format CHECK (btrim(seller_sku) = seller_sku AND seller_sku <> ''),
    ADD CONSTRAINT offers_pack_size_positive CHECK (pack_size > 0),
    ADD CONSTRAINT offers_retail_sale_mode_valid CHECK (retail_sale_mode IN ('piece', 'pack')),
    ADD CONSTRAINT offers_price_required CHECK (price_tiyn IS NOT NULL OR (NOT is_active AND owner_state = 'draft')),
    ADD CONSTRAINT offers_published_complete CHECK (owner_state <> 'published'
        OR (price_tiyn IS NOT NULL AND seller_sku IS NOT NULL AND vat_status IS NOT NULL));
CREATE UNIQUE INDEX offers_seller_sku_key ON offers (supplier_company_id, lower(seller_sku));

-- Closed prices live apart from offers: public queries never touch this table.
CREATE TABLE offer_price_tiers (
    offer_id       BIGINT         NOT NULL REFERENCES offers (id) ON DELETE CASCADE,
    level          TEXT           NOT NULL,
    price_tiyn     BIGINT         NOT NULL,
    sale_mode      TEXT           NOT NULL,
    min_quantity   NUMERIC(14, 3) NOT NULL,
    step_quantity  NUMERIC(14, 3) NOT NULL,
    PRIMARY KEY (offer_id, level),
    CONSTRAINT offer_price_tiers_level_valid CHECK (level IN ('wholesale', 'large_wholesale')),
    CONSTRAINT offer_price_tiers_price_positive CHECK (price_tiyn > 0),
    CONSTRAINT offer_price_tiers_mode_valid CHECK (sale_mode IN ('piece', 'pack'))
);

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION offers_guard() RETURNS trigger AS $$
DECLARE
    card RECORD;
    demo_company BOOLEAN;
BEGIN
    IF TG_OP = 'UPDATE' AND (NEW.is_demo <> OLD.is_demo OR NEW.product_id <> OLD.product_id OR NEW.supplier_company_id <> OLD.supplier_company_id) THEN
        RAISE EXCEPTION 'offer % cannot change its card, company or demo flag', OLD.id USING ERRCODE = 'check_violation';
    END IF;
    SELECT unit, is_demo INTO card FROM products WHERE id = NEW.product_id;
    IF TG_OP = 'INSERT' THEN
        SELECT is_demo INTO demo_company FROM companies WHERE id = NEW.supplier_company_id;
        IF NEW.is_demo IS DISTINCT FROM demo_company OR (NOT NEW.is_demo AND card.is_demo) THEN
            RAISE EXCEPTION 'demo and real data cannot be mixed in an offer' USING ERRCODE = 'check_violation';
        END IF;
    END IF;
    IF NOT NEW.is_demo THEN
        IF NOT sale_rule_ok(card.unit, NEW.retail_sale_mode, NEW.pack_size, NEW.min_order_quantity, NEW.retail_step)
            OR (unit_is_discrete(card.unit) AND NEW.stock_quantity <> trunc(NEW.stock_quantity)) THEN
            RAISE EXCEPTION 'offer quantities do not fit the unit or the pack' USING ERRCODE = 'check_violation';
        END IF;
        IF TG_OP = 'UPDATE' AND EXISTS (SELECT 1 FROM offer_price_tiers t WHERE t.offer_id = NEW.id
            AND NOT sale_rule_ok(card.unit, t.sale_mode, NEW.pack_size, t.min_quantity, t.step_quantity)) THEN
            RAISE EXCEPTION 'price tiers do not fit the pack' USING ERRCODE = 'check_violation';
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- Not fired by unrelated updates, so rows that predate the rules stay untouched.
CREATE TRIGGER offers_guard
    BEFORE INSERT OR UPDATE OF is_demo, product_id, supplier_company_id, owner_state, pack_size, retail_sale_mode, retail_step, min_order_quantity, stock_quantity
    ON offers FOR EACH ROW EXECUTE FUNCTION offers_guard();

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION offer_price_tiers_guard() RETURNS trigger AS $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM offers o JOIN products p ON p.id = o.product_id
        WHERE o.id = NEW.offer_id AND NOT o.is_demo
            AND sale_rule_ok(p.unit, NEW.sale_mode, o.pack_size, NEW.min_quantity, NEW.step_quantity)) THEN
        RAISE EXCEPTION 'price tier quantities do not fit the unit or the pack' USING ERRCODE = 'check_violation';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER offer_price_tiers_guard
    BEFORE INSERT OR UPDATE ON offer_price_tiers
    FOR EACH ROW EXECUTE FUNCTION offer_price_tiers_guard();

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION demo_flag_immutable() RETURNS trigger AS $$
BEGIN
    IF NEW.is_demo <> OLD.is_demo THEN
        RAISE EXCEPTION 'the demo flag cannot change' USING ERRCODE = 'check_violation';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION products_linked_guard() RETURNS trigger AS $$
BEGIN
    IF (NEW.unit <> OLD.unit OR (OLD.gtin IS NOT NULL AND NEW.gtin IS DISTINCT FROM OLD.gtin))
        AND EXISTS (SELECT 1 FROM offers WHERE product_id = OLD.id AND NOT is_demo) THEN
        RAISE EXCEPTION 'product % has linked offers: unit and GTIN are locked', OLD.id USING ERRCODE = 'check_violation';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER companies_demo_immutable BEFORE UPDATE OF is_demo ON companies FOR EACH ROW EXECUTE FUNCTION demo_flag_immutable();
CREATE TRIGGER products_demo_immutable BEFORE UPDATE OF is_demo ON products FOR EACH ROW EXECUTE FUNCTION demo_flag_immutable();
CREATE TRIGGER products_linked_guard BEFORE UPDATE OF unit, gtin ON products FOR EACH ROW EXECUTE FUNCTION products_linked_guard();

-- Append-only history with old and new values. Company data: contains closed prices.
CREATE TABLE offer_events (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    offer_id       BIGINT      NOT NULL,
    company_id     BIGINT      NOT NULL,
    product_id     BIGINT      NOT NULL,
    actor_user_id  BIGINT      NOT NULL,
    action         TEXT        NOT NULL,
    version        INTEGER     NOT NULL,
    details        JSONB       NOT NULL DEFAULT '{}'::jsonb,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT offer_events_action_valid CHECK (action IN ('created', 'updated', 'published', 'hidden'))
);
CREATE INDEX offer_events_offer_idx ON offer_events (offer_id, id);

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION offer_events_append_only() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'offer_events is append-only' USING ERRCODE = 'insufficient_privilege';
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER offer_events_append_only
    BEFORE UPDATE OR DELETE ON offer_events
    FOR EACH ROW EXECUTE FUNCTION offer_events_append_only();

-- +goose Down
-- Real offers stay non-public, and price_tiyn stays nullable: drafts may have no price.
DROP TABLE IF EXISTS offer_events;
DROP FUNCTION IF EXISTS offer_events_append_only();
DROP TRIGGER IF EXISTS products_linked_guard ON products;
DROP TRIGGER IF EXISTS products_demo_immutable ON products;
DROP TRIGGER IF EXISTS companies_demo_immutable ON companies;
DROP FUNCTION IF EXISTS products_linked_guard();
DROP FUNCTION IF EXISTS demo_flag_immutable();
DROP TABLE IF EXISTS offer_price_tiers;
DROP FUNCTION IF EXISTS offer_price_tiers_guard();
DROP TRIGGER IF EXISTS offers_guard ON offers;
DROP FUNCTION IF EXISTS offers_guard();
ALTER TABLE offers
    DROP CONSTRAINT IF EXISTS offers_published_complete,
    DROP CONSTRAINT IF EXISTS offers_price_required,
    DROP CONSTRAINT IF EXISTS offers_real_not_legacy_public,
    DROP COLUMN IF EXISTS version,
    DROP COLUMN IF EXISTS retail_step,
    DROP COLUMN IF EXISTS retail_sale_mode,
    DROP COLUMN IF EXISTS pack_size,
    DROP COLUMN IF EXISTS vat_status,
    DROP COLUMN IF EXISTS seller_sku,
    DROP COLUMN IF EXISTS owner_state;
DROP FUNCTION IF EXISTS sale_rule_ok(TEXT, TEXT, NUMERIC, NUMERIC, NUMERIC);
DROP FUNCTION IF EXISTS unit_is_discrete(TEXT);
