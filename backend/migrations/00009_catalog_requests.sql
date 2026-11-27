-- +goose Up
-- Permissions are not roles: they are granted separately and combine with any role.
CREATE TABLE platform_permissions (
    user_id     BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    permission  TEXT        NOT NULL,
    granted_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, permission),
    CONSTRAINT platform_permissions_valid CHECK (permission IN ('catalog.moderate'))
);

-- Canonical form: 14 digits with leading zeros and a correct check digit.
-- Only built-in functions, so dumps restore under any search_path.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION gtin14_valid(v TEXT) RETURNS BOOLEAN AS $$
    SELECT v ~ '^[0-9]{14}$' AND v <> '00000000000000' AND (
        SELECT sum(substr(v, i, 1)::int * CASE WHEN i % 2 = 1 THEN 3 ELSE 1 END) % 10 = 0
        FROM generate_series(1, 14) AS i)
$$ LANGUAGE sql IMMUTABLE;
-- +goose StatementEnd

-- The catalogue number is assigned by the database and cannot be changed.
-- Numbers are unique but may have gaps.
ALTER TABLE products
    ADD COLUMN catalog_seq        BIGINT GENERATED ALWAYS AS IDENTITY,
    ADD COLUMN gtin               TEXT,
    ADD COLUMN gtin_source_kind   TEXT,
    ADD COLUMN gtin_source        TEXT,
    ADD COLUMN gtin_confirmed_at  TIMESTAMPTZ,
    -- No foreign key: the record of who confirmed outlives the account.
    ADD COLUMN gtin_confirmed_by  BIGINT,
    -- Existing and demo cards have no confirmed GTIN and need a review.
    ADD COLUMN needs_review       BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE products
    ADD COLUMN catalog_no TEXT GENERATED ALWAYS AS
        ('QB' || CASE WHEN length(catalog_seq::text) < 7 THEN lpad(catalog_seq::text, 7, '0') ELSE catalog_seq::text END) STORED,
    ADD CONSTRAINT products_catalog_no_key UNIQUE (catalog_no),
    ADD CONSTRAINT products_gtin_key UNIQUE (gtin),
    ADD CONSTRAINT products_gtin_valid CHECK (gtin IS NULL OR gtin14_valid(gtin)),
    ADD CONSTRAINT products_gtin_confirmed CHECK (
        num_nonnulls(gtin, gtin_source_kind, gtin_source, gtin_confirmed_at, gtin_confirmed_by) IN (0, 5)),
    ADD CONSTRAINT products_gtin_source_kind_valid CHECK (gtin_source_kind IN ('manufacturer', 'packaging', 'official_catalog')),
    ADD CONSTRAINT products_reviewed_has_gtin CHECK (needs_review OR gtin IS NOT NULL);

-- A supplier's request for a new shared card. Drafts may be incomplete.
CREATE TABLE catalog_requests (
    id                  BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    company_id          BIGINT      NOT NULL REFERENCES companies (id) ON DELETE RESTRICT,
    author_user_id      BIGINT      NOT NULL,
    status              TEXT        NOT NULL DEFAULT 'draft',
    submission_no       INTEGER     NOT NULL DEFAULT 0,
    category_id         BIGINT      REFERENCES categories (id) ON DELETE RESTRICT,
    name_kk             TEXT        NOT NULL DEFAULT '',
    name_ru             TEXT        NOT NULL DEFAULT '',
    brand               TEXT        NOT NULL DEFAULT '',
    unit                TEXT        NOT NULL DEFAULT '',
    description_kk      TEXT        NOT NULL DEFAULT '',
    description_ru      TEXT        NOT NULL DEFAULT '',
    -- Declared by the supplier; not confirmed until gtin_confirmed_at is set.
    gtin                TEXT,
    gtin_source_kind    TEXT,
    gtin_source         TEXT        NOT NULL DEFAULT '',
    -- What the moderator checked; may differ from what the supplier named.
    confirmed_source_kind TEXT,
    confirmed_source    TEXT,
    gtin_confirmed_at   TIMESTAMPTZ,
    gtin_confirmed_by   BIGINT,
    comment             TEXT        NOT NULL DEFAULT '',
    product_id          BIGINT      REFERENCES products (id) ON DELETE RESTRICT,
    submitted_at        TIMESTAMPTZ,
    decided_at          TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT catalog_requests_status_valid CHECK (status IN ('draft', 'in_review', 'needs_changes', 'approved', 'rejected')),
    CONSTRAINT catalog_requests_unit_valid CHECK (unit IN ('', 'piece', 'kg', 'tonne', 'bag', 'm', 'm2', 'm3', 'litre', 'pack', 'roll', 'sheet', 'pallet')),
    CONSTRAINT catalog_requests_gtin_valid CHECK (gtin IS NULL OR gtin14_valid(gtin)),
    CONSTRAINT catalog_requests_source_kind_valid CHECK (gtin_source_kind IN ('manufacturer', 'packaging', 'official_catalog')),
    CONSTRAINT catalog_requests_confirmed_kind_valid CHECK (confirmed_source_kind IN ('manufacturer', 'packaging', 'official_catalog')),
    CONSTRAINT catalog_requests_confirmed CHECK (
        num_nonnulls(confirmed_source_kind, confirmed_source, gtin_confirmed_at, gtin_confirmed_by) IN (0, 4)),
    CONSTRAINT catalog_requests_complete CHECK (status = 'draft' OR (
        submission_no > 0 AND submitted_at IS NOT NULL AND (status = 'needs_changes' OR (
        category_id IS NOT NULL AND btrim(name_kk) <> '' AND btrim(name_ru) <> '' AND unit <> ''
        AND gtin IS NOT NULL AND gtin_source_kind IS NOT NULL AND btrim(gtin_source) <> '')))),
    CONSTRAINT catalog_requests_comment_required CHECK (status NOT IN ('needs_changes', 'rejected') OR btrim(comment) <> ''),
    CONSTRAINT catalog_requests_approved CHECK (
        (status = 'approved') = (product_id IS NOT NULL) AND (status <> 'approved' OR gtin_confirmed_at IS NOT NULL))
);
CREATE INDEX catalog_requests_company_idx ON catalog_requests (company_id, id);
CREATE INDEX catalog_requests_status_idx ON catalog_requests (status, submitted_at);
CREATE INDEX catalog_requests_gtin_idx ON catalog_requests (gtin) WHERE status = 'in_review';

CREATE TRIGGER catalog_requests_set_updated_at
    BEFORE UPDATE ON catalog_requests
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION catalog_requests_guard() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF NEW.status <> 'draft' THEN
            RAISE EXCEPTION 'a catalogue request starts as a draft' USING ERRCODE = 'check_violation';
        END IF;
        RETURN NEW;
    END IF;
    IF NEW.company_id <> OLD.company_id OR NEW.author_user_id <> OLD.author_user_id
        OR (NEW.status <> OLD.status AND (OLD.status, NEW.status) NOT IN (
            ('draft', 'in_review'), ('needs_changes', 'in_review'),
            ('in_review', 'approved'), ('in_review', 'needs_changes'), ('in_review', 'rejected'))) THEN
        RAISE EXCEPTION 'catalogue request cannot change from % to %', OLD.status, NEW.status USING ERRCODE = 'check_violation';
    END IF;
    -- What the moderator sees cannot change under review or after the decision.
    IF OLD.status IN ('in_review', 'approved', 'rejected') AND
        (NEW.category_id, NEW.name_kk, NEW.name_ru, NEW.brand, NEW.unit, NEW.description_kk, NEW.description_ru, NEW.gtin, NEW.gtin_source_kind, NEW.gtin_source)
        IS DISTINCT FROM
        (OLD.category_id, OLD.name_kk, OLD.name_ru, OLD.brand, OLD.unit, OLD.description_kk, OLD.description_ru, OLD.gtin, OLD.gtin_source_kind, OLD.gtin_source) THEN
        RAISE EXCEPTION 'catalogue request % is locked', OLD.id USING ERRCODE = 'check_violation';
    END IF;
    -- A confirmation belongs to one exact GTIN.
    IF NEW.gtin IS DISTINCT FROM OLD.gtin THEN
        NEW.confirmed_source_kind := NULL;
        NEW.confirmed_source := NULL;
        NEW.gtin_confirmed_at := NULL;
        NEW.gtin_confirmed_by := NULL;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER catalog_requests_guard
    BEFORE INSERT OR UPDATE ON catalog_requests
    FOR EACH ROW EXECUTE FUNCTION catalog_requests_guard();

-- Append-only history. No foreign keys: it outlives what it describes.
CREATE TABLE catalog_events (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    request_id     BIGINT      NOT NULL,
    company_id     BIGINT      NOT NULL,
    product_id     BIGINT,
    submission_no  INTEGER     NOT NULL,
    actor_user_id  BIGINT      NOT NULL,
    actor_name     TEXT        NOT NULL,
    action         TEXT        NOT NULL,
    comment        TEXT        NOT NULL DEFAULT '',
    details        JSONB       NOT NULL DEFAULT '{}'::jsonb,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT catalog_events_action_valid CHECK (action IN
        ('created', 'submitted', 'gtin_confirmed', 'changes_requested', 'approved', 'rejected'))
);
CREATE INDEX catalog_events_request_idx ON catalog_events (request_id, id);

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION catalog_events_append_only() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'catalog_events is append-only' USING ERRCODE = 'insufficient_privilege';
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER catalog_events_append_only
    BEFORE UPDATE OR DELETE ON catalog_events
    FOR EACH ROW EXECUTE FUNCTION catalog_events_append_only();

-- +goose Down
DROP TABLE IF EXISTS catalog_events;
DROP FUNCTION IF EXISTS catalog_events_append_only();
DROP TABLE IF EXISTS catalog_requests;
DROP FUNCTION IF EXISTS catalog_requests_guard();
ALTER TABLE products
    DROP COLUMN IF EXISTS catalog_no,
    DROP CONSTRAINT IF EXISTS products_reviewed_has_gtin,
    DROP CONSTRAINT IF EXISTS products_gtin_confirmed,
    DROP CONSTRAINT IF EXISTS products_gtin_valid,
    DROP COLUMN IF EXISTS needs_review,
    DROP COLUMN IF EXISTS gtin_confirmed_by,
    DROP COLUMN IF EXISTS gtin_confirmed_at,
    DROP COLUMN IF EXISTS gtin_source,
    DROP COLUMN IF EXISTS gtin_source_kind,
    DROP COLUMN IF EXISTS gtin,
    DROP COLUMN IF EXISTS catalog_seq;
DROP FUNCTION IF EXISTS gtin14_valid(TEXT);
DROP TABLE IF EXISTS platform_permissions;
