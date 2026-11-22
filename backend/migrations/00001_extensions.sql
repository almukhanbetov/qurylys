-- +goose Up
-- pg_trgm speeds up ILIKE '%term%' search on product names.
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- Shared trigger function that keeps updated_at current.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION IF EXISTS set_updated_at();
-- The extension is left in place on purpose: other objects in the database may use it.
