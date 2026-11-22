-- +goose Up
-- Product is the shared catalogue card. Prices, stock and delivery live in offers.
CREATE TABLE products (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    category_id     BIGINT      NOT NULL REFERENCES categories (id) ON DELETE RESTRICT,
    slug            TEXT        NOT NULL,
    name_kk         TEXT        NOT NULL,
    name_ru         TEXT        NOT NULL,
    description_kk  TEXT        NOT NULL DEFAULT '',
    description_ru  TEXT        NOT NULL DEFAULT '',
    brand           TEXT,
    -- Unit of measure code; the UI translates it. Keep this list in sync with
    -- backend/internal/products/model.go and frontend dictionaries.
    unit            TEXT        NOT NULL,
    -- Characteristics: JSON array of
    --   {"key": "...", "name_kk": "...", "name_ru": "...", "value": "..."}
    attributes      JSONB       NOT NULL DEFAULT '[]'::jsonb,
    is_active       BOOLEAN     NOT NULL DEFAULT TRUE,
    is_demo         BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT products_slug_key UNIQUE (slug),
    CONSTRAINT products_slug_format CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
    CONSTRAINT products_names_not_blank CHECK (btrim(name_kk) <> '' AND btrim(name_ru) <> ''),
    CONSTRAINT products_unit_valid CHECK (unit IN ('piece', 'kg', 'tonne', 'bag', 'm', 'm2', 'm3', 'litre', 'pack', 'roll', 'sheet', 'pallet')),
    CONSTRAINT products_attributes_array CHECK (jsonb_typeof(attributes) = 'array')
);

CREATE INDEX products_category_idx ON products (category_id) WHERE is_active;
CREATE INDEX products_created_idx  ON products (created_at DESC, id DESC) WHERE is_active;
-- Trigram indexes back the case-insensitive substring search.
CREATE INDEX products_name_kk_trgm_idx ON products USING gin (name_kk gin_trgm_ops);
CREATE INDEX products_name_ru_trgm_idx ON products USING gin (name_ru gin_trgm_ops);
CREATE INDEX products_brand_trgm_idx   ON products USING gin (brand gin_trgm_ops);

CREATE TRIGGER products_set_updated_at
    BEFORE UPDATE ON products
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE IF EXISTS products;
