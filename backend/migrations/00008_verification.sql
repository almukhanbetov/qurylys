-- +goose Up
-- One application for company verification. submission_no counts submissions:
-- it lets reviewers decide only on the version they saw.
CREATE TABLE verification_requests (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    company_id     BIGINT      NOT NULL REFERENCES companies (id) ON DELETE RESTRICT,
    status         TEXT        NOT NULL DEFAULT 'draft',
    submission_no  INTEGER     NOT NULL DEFAULT 0,
    -- Requisites exactly as submitted; reviewers see these, not the live row.
    snapshot       JSONB,
    -- Latest reviewer comment.
    comment        TEXT        NOT NULL DEFAULT '',
    submitted_at   TIMESTAMPTZ,
    decided_at     TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT verification_requests_company_key UNIQUE (id, company_id),
    CONSTRAINT verification_requests_status_valid
        CHECK (status IN ('draft', 'in_review', 'needs_changes', 'approved', 'rejected')),
    CONSTRAINT verification_requests_submitted
        CHECK ((status = 'draft') = (submission_no = 0)
            AND (status = 'draft' OR (snapshot IS NOT NULL AND submitted_at IS NOT NULL))),
    CONSTRAINT verification_requests_comment_required
        CHECK (status NOT IN ('needs_changes', 'rejected') OR btrim(comment) <> '')
);
CREATE UNIQUE INDEX verification_requests_open_key ON verification_requests (company_id)
    WHERE status IN ('draft', 'in_review', 'needs_changes');
CREATE INDEX verification_requests_status_idx ON verification_requests (status, submitted_at);

CREATE TRIGGER verification_requests_set_updated_at
    BEFORE UPDATE ON verification_requests
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION verification_requests_guard() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF NEW.status <> 'draft' THEN
            RAISE EXCEPTION 'a verification request starts as a draft' USING ERRCODE = 'check_violation';
        END IF;
        RETURN NEW;
    END IF;
    IF NEW.company_id <> OLD.company_id OR (NEW.status <> OLD.status AND (OLD.status, NEW.status) NOT IN (
        ('draft', 'in_review'), ('needs_changes', 'in_review'),
        ('in_review', 'approved'), ('in_review', 'needs_changes'), ('in_review', 'rejected')
    )) THEN
        RAISE EXCEPTION 'verification status cannot change from % to %', OLD.status, NEW.status
            USING ERRCODE = 'check_violation';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER verification_requests_guard
    BEFORE INSERT OR UPDATE ON verification_requests
    FOR EACH ROW EXECUTE FUNCTION verification_requests_guard();

-- Private files: reachable only through authorized API handlers, included in
-- the regular database backup.
CREATE TABLE verification_documents (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    request_id     BIGINT      NOT NULL REFERENCES verification_requests (id) ON DELETE RESTRICT,
    kind           TEXT        NOT NULL,
    filename       TEXT        NOT NULL,
    content_type   TEXT        NOT NULL,
    size           INTEGER     NOT NULL,
    sha256         BYTEA       NOT NULL,
    data           BYTEA       NOT NULL,
    -- submission_no of the request when the file was added / removed. A file is
    -- part of submission N when added_after < N and (removed_after IS NULL OR
    -- removed_after >= N).
    added_after    INTEGER     NOT NULL,
    removed_after  INTEGER,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT verification_documents_kind_valid CHECK (kind IN ('registration', 'authority', 'other')),
    CONSTRAINT verification_documents_type_valid CHECK (content_type IN ('application/pdf', 'image/jpeg', 'image/png')),
    CONSTRAINT verification_documents_size_valid CHECK (size = octet_length(data) AND size BETWEEN 1 AND 5242880)
);
-- The files are already compressed.
ALTER TABLE verification_documents ALTER COLUMN data SET STORAGE EXTERNAL;
CREATE INDEX verification_documents_request_idx ON verification_documents (request_id);

-- Who did what, when and why. Append-only, no foreign keys: history outlives the rest.
CREATE TABLE verification_events (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    request_id     BIGINT      NOT NULL,
    company_id     BIGINT      NOT NULL,
    submission_no  INTEGER     NOT NULL,
    actor_user_id  BIGINT      NOT NULL,
    actor_name     TEXT        NOT NULL,
    action         TEXT        NOT NULL,
    from_status    TEXT,
    to_status      TEXT,
    comment        TEXT        NOT NULL DEFAULT '',
    -- Submissions: requisites and document hashes. Changes: old and new values.
    details        JSONB       NOT NULL DEFAULT '{}'::jsonb,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT verification_events_action_valid CHECK (action IN
        ('created', 'submitted', 'approved', 'changes_requested', 'rejected', 'verification_lost'))
);
CREATE INDEX verification_events_company_idx ON verification_events (company_id, id);

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION verification_events_append_only() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'verification_events is append-only' USING ERRCODE = 'insufficient_privilege';
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER verification_events_append_only
    BEFORE UPDATE OR DELETE ON verification_events
    FOR EACH ROW EXECUTE FUNCTION verification_events_append_only();

-- The approved request that currently holds; NULL = not verified.
ALTER TABLE companies
    ADD COLUMN verified_request_id BIGINT,
    ADD CONSTRAINT companies_verified_request FOREIGN KEY (verified_request_id, id)
        REFERENCES verification_requests (id, company_id) ON DELETE RESTRICT;

-- Last line of defence behind the service: no code path can keep verification
-- after a key change, change key data under review, or verify without approval.
-- Key data: name, BIN, legal address, representative, becoming a supplier.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION companies_verification_guard() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF NEW.verified_request_id IS NOT NULL THEN
            RAISE EXCEPTION 'a new company cannot be verified' USING ERRCODE = 'check_violation';
        END IF;
        RETURN NEW;
    END IF;
    IF (NEW.name, NEW.bin, NEW.legal_address, NEW.representative_name)
            IS DISTINCT FROM (OLD.name, OLD.bin, OLD.legal_address, OLD.representative_name)
        OR (NEW.is_supplier AND NOT OLD.is_supplier) THEN
        IF EXISTS (SELECT 1 FROM verification_requests WHERE company_id = NEW.id AND status = 'in_review') THEN
            RAISE EXCEPTION 'company % is under review', NEW.id USING ERRCODE = 'check_violation';
        END IF;
        NEW.verified_request_id := NULL;
    END IF;
    IF NEW.verified_request_id IS NOT NULL AND NEW.verified_request_id IS DISTINCT FROM OLD.verified_request_id
        AND NOT EXISTS (SELECT 1 FROM verification_requests
            WHERE id = NEW.verified_request_id AND company_id = NEW.id AND status = 'approved') THEN
        RAISE EXCEPTION 'company % has no such approved request', NEW.id USING ERRCODE = 'check_violation';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER companies_verification_guard
    BEFORE INSERT OR UPDATE ON companies
    FOR EACH ROW EXECUTE FUNCTION companies_verification_guard();

-- The single definition of the status shown to the company.
CREATE VIEW company_verification AS
SELECT c.id AS company_id,
    CASE
        WHEN c.verified_request_id IS NOT NULL THEN 'approved'
        WHEN r.id IS NULL THEN 'none'
        WHEN r.status = 'approved' THEN 'reverification'
        ELSE r.status
    END AS status,
    r.id AS request_id
FROM companies c
LEFT JOIN LATERAL (
    SELECT id, status FROM verification_requests WHERE company_id = c.id ORDER BY id DESC LIMIT 1
) r ON TRUE;

-- +goose Down
DROP VIEW IF EXISTS company_verification;
DROP TRIGGER IF EXISTS companies_verification_guard ON companies;
DROP FUNCTION IF EXISTS companies_verification_guard();
ALTER TABLE companies
    DROP CONSTRAINT IF EXISTS companies_verified_request,
    DROP COLUMN IF EXISTS verified_request_id;
DROP TABLE IF EXISTS verification_events;
DROP FUNCTION IF EXISTS verification_events_append_only();
DROP TABLE IF EXISTS verification_documents;
DROP TABLE IF EXISTS verification_requests;
DROP FUNCTION IF EXISTS verification_requests_guard();
