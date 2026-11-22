-- +goose Up
CREATE TABLE companies (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    -- Stable machine key used by the demo seed for idempotent upserts and in URLs.
    slug         TEXT        NOT NULL,
    name         TEXT        NOT NULL,
    -- БСН/BIN: 12 digits. Optional until company verification exists (stage 2+).
    bin          CHAR(12),
    city         TEXT,
    -- A company can be a buyer, a supplier, or both. Never neither.
    is_buyer     BOOLEAN     NOT NULL DEFAULT FALSE,
    is_supplier  BOOLEAN     NOT NULL DEFAULT FALSE,
    status       TEXT        NOT NULL DEFAULT 'active',
    -- Demo rows are created only by the seed command and must be labelled in the UI.
    is_demo      BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT companies_slug_key   UNIQUE (slug),
    CONSTRAINT companies_bin_key    UNIQUE (bin),
    CONSTRAINT companies_slug_format CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
    CONSTRAINT companies_name_not_blank CHECK (btrim(name) <> ''),
    CONSTRAINT companies_bin_format CHECK (bin IS NULL OR bin ~ '^[0-9]{12}$'),
    CONSTRAINT companies_has_role   CHECK (is_buyer OR is_supplier),
    CONSTRAINT companies_status_valid CHECK (status IN ('active', 'suspended'))
);

CREATE INDEX companies_suppliers_idx ON companies (id) WHERE is_supplier AND status = 'active';

CREATE TRIGGER companies_set_updated_at
    BEFORE UPDATE ON companies
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE IF EXISTS companies;
