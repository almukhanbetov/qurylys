-- +goose Up
-- Additive: existing companies keep their rows and get NULL details.
ALTER TABLE companies
    ADD COLUMN legal_address        TEXT,
    ADD COLUMN actual_address       TEXT,
    ADD COLUMN work_phone           TEXT,
    ADD COLUMN representative_name  TEXT,
    ADD CONSTRAINT companies_work_phone_format CHECK (work_phone IS NULL OR work_phone ~ '^\+7[0-9]{10}$');

CREATE TABLE company_members (
    company_id  BIGINT      NOT NULL REFERENCES companies (id) ON DELETE CASCADE,
    user_id     BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role        TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (company_id, user_id),
    CONSTRAINT company_members_role_valid CHECK (role IN ('owner', 'employee'))
);
CREATE INDEX company_members_user_idx ON company_members (user_id);

-- The active company is per session. The composite key guarantees that it is
-- always a company the user belongs to; losing membership clears it.
ALTER TABLE sessions
    ADD COLUMN active_company_id BIGINT,
    ADD CONSTRAINT sessions_active_membership
        FOREIGN KEY (active_company_id, user_id) REFERENCES company_members (company_id, user_id)
        ON DELETE SET NULL (active_company_id);
CREATE INDEX sessions_active_company_idx ON sessions (active_company_id, user_id);

-- Addressed to a contact, not to a user, so inviting never reveals who is registered.
CREATE TABLE company_invitations (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    company_id    BIGINT      NOT NULL REFERENCES companies (id) ON DELETE CASCADE,
    contact_type  TEXT        NOT NULL,
    contact       TEXT        NOT NULL,
    status        TEXT        NOT NULL DEFAULT 'pending',
    invited_by    BIGINT      REFERENCES users (id) ON DELETE SET NULL,
    expires_at    TIMESTAMPTZ NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT company_invitations_contact_type_valid CHECK (contact_type IN ('email', 'phone')),
    CONSTRAINT company_invitations_status_valid CHECK (status IN ('pending', 'accepted', 'declined', 'cancelled'))
);
CREATE UNIQUE INDEX company_invitations_pending_key ON company_invitations (company_id, contact) WHERE status = 'pending';
CREATE INDEX company_invitations_contact_idx ON company_invitations (contact) WHERE status = 'pending';

-- +goose Down
DROP TABLE IF EXISTS company_invitations;
ALTER TABLE sessions
    DROP CONSTRAINT IF EXISTS sessions_active_membership,
    DROP COLUMN IF EXISTS active_company_id;
DROP TABLE IF EXISTS company_members;
ALTER TABLE companies
    DROP CONSTRAINT IF EXISTS companies_work_phone_format,
    DROP COLUMN IF EXISTS legal_address,
    DROP COLUMN IF EXISTS actual_address,
    DROP COLUMN IF EXISTS work_phone,
    DROP COLUMN IF EXISTS representative_name;
