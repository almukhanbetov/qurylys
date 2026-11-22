-- +goose Up
CREATE TABLE categories (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    -- NULL parent means a root category. RESTRICT: a category with children
    -- cannot be deleted until the children are moved or removed.
    parent_id   BIGINT REFERENCES categories (id) ON DELETE RESTRICT,
    slug        TEXT        NOT NULL,
    name_kk     TEXT        NOT NULL,
    name_ru     TEXT        NOT NULL,
    sort_order  INTEGER     NOT NULL DEFAULT 0,
    is_active   BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT categories_slug_key UNIQUE (slug),
    CONSTRAINT categories_slug_format CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
    CONSTRAINT categories_not_own_parent CHECK (parent_id IS NULL OR parent_id <> id),
    CONSTRAINT categories_names_not_blank CHECK (btrim(name_kk) <> '' AND btrim(name_ru) <> '')
);

CREATE INDEX categories_parent_idx ON categories (parent_id, sort_order);

-- Longer cycles (A -> B -> A) cannot be expressed as a CHECK, so a trigger
-- walks up from the new parent and rejects the write if it reaches the row itself.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION categories_prevent_cycle() RETURNS trigger AS $$
BEGIN
    IF NEW.parent_id IS NULL THEN
        RETURN NEW;
    END IF;
    IF EXISTS (
        WITH RECURSIVE ancestors AS (
            SELECT id, parent_id FROM categories WHERE id = NEW.parent_id
            UNION
            SELECT c.id, c.parent_id FROM categories c JOIN ancestors a ON c.id = a.parent_id
        )
        SELECT 1 FROM ancestors WHERE id = NEW.id
    ) THEN
        RAISE EXCEPTION 'category % cannot be placed under its own descendant %', NEW.id, NEW.parent_id
            USING ERRCODE = 'check_violation';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER categories_prevent_cycle
    BEFORE INSERT OR UPDATE OF parent_id ON categories
    FOR EACH ROW EXECUTE FUNCTION categories_prevent_cycle();

CREATE TRIGGER categories_set_updated_at
    BEFORE UPDATE ON categories
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE IF EXISTS categories;
DROP FUNCTION IF EXISTS categories_prevent_cycle();
