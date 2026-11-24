-- +goose Up
-- A user exists only after the chosen contact has been confirmed, so every
-- stored contact is a confirmed one. Pending registrations live in auth_challenges.
CREATE TABLE users (
    id                 BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    full_name          TEXT        NOT NULL,
    email              TEXT,
    email_verified_at  TIMESTAMPTZ,
    phone              TEXT,
    phone_verified_at  TIMESTAMPTZ,
    -- argon2id in PHC string format.
    password_hash      TEXT        NOT NULL,
    status             TEXT        NOT NULL DEFAULT 'active',
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT users_email_key UNIQUE (email),
    CONSTRAINT users_phone_key UNIQUE (phone),
    CONSTRAINT users_full_name_not_blank CHECK (btrim(full_name) <> ''),
    CONSTRAINT users_email_format CHECK (email IS NULL OR (email = lower(email) AND email LIKE '%_@_%')),
    CONSTRAINT users_phone_format CHECK (phone IS NULL OR phone ~ '^\+7[0-9]{10}$'),
    CONSTRAINT users_has_contact CHECK (email IS NOT NULL OR phone IS NOT NULL),
    CONSTRAINT users_email_confirmed CHECK ((email IS NULL) = (email_verified_at IS NULL)),
    CONSTRAINT users_phone_confirmed CHECK ((phone IS NULL) = (phone_verified_at IS NULL)),
    CONSTRAINT users_status_valid CHECK (status IN ('active', 'blocked'))
);

CREATE TRIGGER users_set_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Platform-level roles, separate from company membership. Never self-assigned:
-- 'admin' is granted only by the admin command, 'mrx' by an administrator.
CREATE TABLE platform_roles (
    user_id     BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role        TEXT        NOT NULL,
    granted_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, role),
    CONSTRAINT platform_roles_valid CHECK (role IN ('mrx', 'admin'))
);

-- Only the SHA-256 of the cookie token is stored.
CREATE TABLE sessions (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id       BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash    BYTEA       NOT NULL UNIQUE,
    user_agent    TEXT        NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at    TIMESTAMPTZ NOT NULL
);
CREATE INDEX sessions_user_idx ON sessions (user_id);
CREATE INDEX sessions_expires_idx ON sessions (expires_at);

-- One-time codes. token_hash = SHA-256 of a random token known only to the
-- client; code_hash = HMAC(token, code), so the table alone reveals no code.
CREATE TABLE auth_challenges (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    token_hash     BYTEA       NOT NULL UNIQUE,
    purpose        TEXT        NOT NULL,
    contact_type   TEXT        NOT NULL,
    contact        TEXT        NOT NULL,
    -- NULL for registrations and for resets of unknown contacts (which can never succeed).
    user_id        BIGINT      REFERENCES users (id) ON DELETE CASCADE,
    -- Pending registration data.
    full_name      TEXT,
    password_hash  TEXT,
    code_hash      BYTEA       NOT NULL,
    attempts       INTEGER     NOT NULL DEFAULT 0,
    expires_at     TIMESTAMPTZ NOT NULL,
    consumed_at    TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT auth_challenges_purpose_valid CHECK (purpose IN ('register', 'reset_password', 'add_contact')),
    CONSTRAINT auth_challenges_contact_type_valid CHECK (contact_type IN ('email', 'phone')),
    CONSTRAINT auth_challenges_register_data CHECK (
        purpose <> 'register' OR (full_name IS NOT NULL AND password_hash IS NOT NULL)
    )
);
CREATE INDEX auth_challenges_expires_idx ON auth_challenges (expires_at);

-- Fixed-window counters; keys are hashed, no contacts or addresses in clear text.
CREATE TABLE rate_limits (
    key         TEXT        PRIMARY KEY,
    count       INTEGER     NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL
);
CREATE INDEX rate_limits_expires_idx ON rate_limits (expires_at);

-- Append-only. No foreign keys on purpose: entries outlive what they describe.
CREATE TABLE audit_events (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    -- NULL: an operator command.
    actor_user_id   BIGINT,
    company_id      BIGINT,
    target_user_id  BIGINT,
    action          TEXT        NOT NULL,
    details         JSONB       NOT NULL DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX audit_events_company_idx ON audit_events (company_id, id);

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION audit_events_append_only() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'audit_events is append-only' USING ERRCODE = 'insufficient_privilege';
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER audit_events_append_only
    BEFORE UPDATE OR DELETE ON audit_events
    FOR EACH ROW EXECUTE FUNCTION audit_events_append_only();

-- +goose Down
DROP TABLE IF EXISTS audit_events;
DROP FUNCTION IF EXISTS audit_events_append_only();
DROP TABLE IF EXISTS rate_limits;
DROP TABLE IF EXISTS auth_challenges;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS platform_roles;
DROP TABLE IF EXISTS users;
