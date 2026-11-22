-- +goose Up
-- Offer is one supplier's commercial terms for one product.
-- A future cart line will reference offers.id, not products.id.
CREATE TABLE offers (
    id                   BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    product_id           BIGINT      NOT NULL REFERENCES products (id) ON DELETE RESTRICT,
    supplier_company_id  BIGINT      NOT NULL REFERENCES companies (id) ON DELETE RESTRICT,
    -- Money: integer number of tiyn (1 KZT = 100 tiyn) per one product unit.
    -- See docs/money.md. Never float.
    price_tiyn           BIGINT      NOT NULL,
    currency             CHAR(3)     NOT NULL DEFAULT 'KZT',
    -- Quantities are in products.unit and may be fractional (e.g. 2.5 m3).
    stock_quantity       NUMERIC(14, 3) NOT NULL DEFAULT 0,
    min_order_quantity   NUMERIC(14, 3) NOT NULL DEFAULT 1,
    delivery_available   BOOLEAN     NOT NULL DEFAULT FALSE,
    delivery_days_min    INTEGER,
    delivery_days_max    INTEGER,
    delivery_terms_kk    TEXT        NOT NULL DEFAULT '',
    delivery_terms_ru    TEXT        NOT NULL DEFAULT '',
    city                 TEXT,
    is_active            BOOLEAN     NOT NULL DEFAULT TRUE,
    is_demo              BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- One offer per supplier per product; price changes update the row.
    CONSTRAINT offers_product_supplier_key UNIQUE (product_id, supplier_company_id),
    CONSTRAINT offers_price_positive CHECK (price_tiyn > 0),
    CONSTRAINT offers_currency_kzt CHECK (currency = 'KZT'),
    CONSTRAINT offers_stock_non_negative CHECK (stock_quantity >= 0),
    CONSTRAINT offers_moq_positive CHECK (min_order_quantity > 0),
    CONSTRAINT offers_delivery_days_valid CHECK (
        (delivery_days_min IS NULL OR delivery_days_min >= 0)
        AND (delivery_days_max IS NULL OR delivery_days_max >= 0)
        AND (delivery_days_min IS NULL OR delivery_days_max IS NULL OR delivery_days_min <= delivery_days_max)
    )
);

CREATE INDEX offers_supplier_idx ON offers (supplier_company_id);
-- Supports the min-price aggregation per product (see docs/catalog.md).
CREATE INDEX offers_product_price_idx ON offers (product_id, price_tiyn) WHERE is_active;

-- Only companies with the supplier role may hold offers.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION offers_require_supplier() RETURNS trigger AS $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM companies WHERE id = NEW.supplier_company_id AND is_supplier) THEN
        RAISE EXCEPTION 'company % is not a supplier', NEW.supplier_company_id
            USING ERRCODE = 'check_violation';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER offers_require_supplier
    BEFORE INSERT OR UPDATE OF supplier_company_id ON offers
    FOR EACH ROW EXECUTE FUNCTION offers_require_supplier();

CREATE TRIGGER offers_set_updated_at
    BEFORE UPDATE ON offers
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE IF EXISTS offers;
DROP FUNCTION IF EXISTS offers_require_supplier();
