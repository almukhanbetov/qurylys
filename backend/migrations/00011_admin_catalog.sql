-- +goose Up
-- Administration of the shared catalogue: categories, brands and cards.
-- Additive only: existing ids, links and brand texts stay as they are.

-- Separate permissions; no role implies them (like catalog.moderate).
ALTER TABLE platform_permissions DROP CONSTRAINT platform_permissions_valid;
ALTER TABLE platform_permissions ADD CONSTRAINT platform_permissions_valid
    CHECK (permission IN ('catalog.moderate', 'catalog.categories', 'catalog.brands'));

-- Optimistic locking: every update, by any path, makes older copies stale.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION bump_version() RETURNS trigger AS $$
BEGIN
    NEW.version := OLD.version + 1;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

ALTER TABLE categories ADD COLUMN version INTEGER NOT NULL DEFAULT 1;
CREATE TRIGGER categories_bump_version BEFORE UPDATE ON categories
    FOR EACH ROW EXECUTE FUNCTION bump_version();

-- Brand / manufacturer directory. Names are stored trimmed with single spaces;
-- two brands cannot differ only in letter case.
CREATE TABLE brands (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name        TEXT        NOT NULL,
    version     INTEGER     NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT brands_name_valid CHECK (
        name <> '' AND name = btrim(name) AND name !~ '\s\s' AND name !~ '[[:cntrl:]]' AND char_length(name) <= 100)
);
CREATE UNIQUE INDEX brands_name_key ON brands (lower(name));
CREATE INDEX brands_name_trgm_idx ON brands USING gin (name gin_trgm_ops);

CREATE TRIGGER brands_set_updated_at BEFORE UPDATE ON brands
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER brands_bump_version BEFORE UPDATE ON brands
    FOR EACH ROW EXECUTE FUNCTION bump_version();

-- products.brand stays the display text that public pages and search read.
-- A linked card shows the brand's name; an unlinked one keeps its old text.
-- Nothing is linked automatically here: similar spellings are not merged.
ALTER TABLE products
    ADD COLUMN brand_id      BIGINT REFERENCES brands (id) ON DELETE RESTRICT,
    ADD COLUMN version       INTEGER NOT NULL DEFAULT 1,
    -- Blocking by a moderator (R6). No foreign key: the record outlives the account.
    ADD COLUMN blocked_at    TIMESTAMPTZ,
    ADD COLUMN block_reason  TEXT,
    ADD COLUMN blocked_by    BIGINT,
    ADD CONSTRAINT products_block_complete CHECK (
        num_nonnulls(blocked_at, block_reason, blocked_by) IN (0, 3) AND (block_reason IS NULL OR btrim(block_reason) <> '')),
    -- Demo cards are maintained by the seed only.
    ADD CONSTRAINT products_demo_no_brand CHECK (NOT is_demo OR (brand_id IS NULL AND blocked_at IS NULL));
CREATE INDEX products_brand_idx ON products (brand_id) WHERE brand_id IS NOT NULL;

CREATE TRIGGER products_bump_version BEFORE UPDATE ON products
    FOR EACH ROW EXECUTE FUNCTION bump_version();

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION products_brand_text() RETURNS trigger AS $$
BEGIN
    IF NEW.brand_id IS NOT NULL THEN
        SELECT name INTO NEW.brand FROM brands WHERE id = NEW.brand_id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION brands_rename_cards() RETURNS trigger AS $$
BEGIN
    UPDATE products SET brand = NEW.name WHERE brand_id = NEW.id AND brand IS DISTINCT FROM NEW.name;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER products_brand_text BEFORE INSERT OR UPDATE OF brand_id, brand ON products
    FOR EACH ROW EXECUTE FUNCTION products_brand_text();
CREATE TRIGGER brands_rename_cards AFTER UPDATE OF name ON brands
    FOR EACH ROW EXECUTE FUNCTION brands_rename_cards();

-- A card with real offers keeps what identifies the product (R11, N3): unit,
-- GTIN, category and an assigned brand. A brand may still be assigned once.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION products_linked_guard() RETURNS trigger AS $$
BEGIN
    IF (NEW.unit <> OLD.unit
        OR (OLD.gtin IS NOT NULL AND NEW.gtin IS DISTINCT FROM OLD.gtin)
        OR NEW.category_id <> OLD.category_id
        OR (OLD.brand_id IS NOT NULL AND NEW.brand_id IS DISTINCT FROM OLD.brand_id))
        AND EXISTS (SELECT 1 FROM offers WHERE product_id = OLD.id AND NOT is_demo) THEN
        RAISE EXCEPTION 'product % has linked offers: unit, GTIN, category and brand are locked', OLD.id USING ERRCODE = 'check_violation';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

DROP TRIGGER products_linked_guard ON products;
CREATE TRIGGER products_linked_guard BEFORE UPDATE OF unit, gtin, category_id, brand_id ON products
    FOR EACH ROW EXECUTE FUNCTION products_linked_guard();

-- The catalogue journal also records actions on cards, without a request.
ALTER TABLE catalog_events
    ALTER COLUMN request_id DROP NOT NULL,
    ALTER COLUMN company_id DROP NOT NULL,
    ALTER COLUMN submission_no DROP NOT NULL,
    DROP CONSTRAINT catalog_events_action_valid,
    ADD CONSTRAINT catalog_events_action_valid CHECK (action IN
        ('created', 'submitted', 'gtin_confirmed', 'changes_requested', 'approved', 'rejected',
         'card_created', 'card_edited', 'card_gtin_confirmed', 'card_blocked', 'card_unblocked')),
    ADD CONSTRAINT catalog_events_subject CHECK (
        (request_id IS NOT NULL AND company_id IS NOT NULL AND submission_no IS NOT NULL)
        OR (request_id IS NULL AND product_id IS NOT NULL AND action LIKE 'card\_%'));
CREATE INDEX catalog_events_product_idx ON catalog_events (product_id, id) WHERE product_id IS NOT NULL;

-- +goose Down
-- The journal is append-only: card events stay, so its wider shape stays too
-- (nullable request columns, card actions). Earlier code only adds request events.
DROP INDEX IF EXISTS catalog_events_product_idx;
ALTER TABLE catalog_events DROP CONSTRAINT IF EXISTS catalog_events_subject;

DROP TRIGGER products_linked_guard ON products;
-- +goose StatementBegin
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
CREATE TRIGGER products_linked_guard BEFORE UPDATE OF unit, gtin ON products
    FOR EACH ROW EXECUTE FUNCTION products_linked_guard();

DROP TRIGGER IF EXISTS brands_rename_cards ON brands;
DROP TRIGGER IF EXISTS products_brand_text ON products;
DROP FUNCTION IF EXISTS brands_rename_cards();
DROP FUNCTION IF EXISTS products_brand_text();
DROP TRIGGER IF EXISTS products_bump_version ON products;
ALTER TABLE products
    DROP CONSTRAINT IF EXISTS products_demo_no_brand,
    DROP CONSTRAINT IF EXISTS products_block_complete,
    DROP COLUMN IF EXISTS blocked_by,
    DROP COLUMN IF EXISTS block_reason,
    DROP COLUMN IF EXISTS blocked_at,
    DROP COLUMN IF EXISTS version,
    DROP COLUMN IF EXISTS brand_id;
DROP TABLE IF EXISTS brands;
DROP TRIGGER IF EXISTS categories_bump_version ON categories;
ALTER TABLE categories DROP COLUMN IF EXISTS version;
DROP FUNCTION IF EXISTS bump_version();
DELETE FROM platform_permissions WHERE permission IN ('catalog.categories', 'catalog.brands');
ALTER TABLE platform_permissions DROP CONSTRAINT platform_permissions_valid;
ALTER TABLE platform_permissions ADD CONSTRAINT platform_permissions_valid CHECK (permission IN ('catalog.moderate'));
