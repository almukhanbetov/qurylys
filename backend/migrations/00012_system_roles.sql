-- +goose Up
-- Reference of the 19 roles of the specification (§02, p. 3-4). This is a
-- directory, not an assignment: it grants nothing. Existing access stays where
-- it is (platform_roles, platform_permissions, company_members, company
-- directions). Codes are fixed; only a migration changes the status.
CREATE TABLE system_roles (
    code          TEXT    PRIMARY KEY,
    contour       TEXT    NOT NULL,
    -- platform: acts across organisations; organization: only inside the
    -- organisation of the assignment (§03, p. 5), never globally.
    scope         TEXT    NOT NULL,
    -- Branch rule from the specification: none (not applicable), all branches,
    -- an assigned branch, or not defined yet.
    branch_scope  TEXT    NOT NULL,
    -- partial: some functions work; planned: only in this directory.
    status        TEXT    NOT NULL,
    sort_order    INTEGER NOT NULL UNIQUE,
    CONSTRAINT system_roles_code_valid CHECK (code IN (
        'MP', 'SL', 'MRX', 'GD', 'BD', 'ACC', 'WH', 'DR', 'SR', 'OP', 'SV', 'LG', 'PR', 'CA', 'AU', 'HR', 'TS', 'CC', 'SA')),
    CONSTRAINT system_roles_contour_valid CHECK (contour IN ('marketplace', 'erp', 'platform')),
    CONSTRAINT system_roles_scope_valid CHECK (scope IN ('platform', 'organization')),
    CONSTRAINT system_roles_branch_valid CHECK (branch_scope IN ('none', 'all', 'assigned', 'unspecified')),
    CONSTRAINT system_roles_status_valid CHECK (status IN ('implemented', 'partial', 'planned')),
    -- ERP roles act inside one organisation only.
    CONSTRAINT system_roles_erp_in_organization CHECK (contour <> 'erp' OR scope = 'organization'),
    CONSTRAINT system_roles_platform_has_no_branch CHECK (scope <> 'platform' OR branch_scope = 'none')
);

INSERT INTO system_roles (code, contour, scope, branch_scope, status, sort_order) VALUES
    ('MP',  'marketplace', 'organization', 'unspecified', 'partial',  1),
    ('SL',  'marketplace', 'organization', 'unspecified', 'partial',  2),
    ('MRX', 'marketplace', 'platform',     'none',        'partial',  3),
    ('GD',  'erp',         'organization', 'all',         'planned',  4),
    ('BD',  'erp',         'organization', 'assigned',    'planned',  5),
    ('ACC', 'erp',         'organization', 'unspecified', 'planned',  6),
    ('WH',  'erp',         'organization', 'unspecified', 'planned',  7),
    ('DR',  'erp',         'organization', 'unspecified', 'planned',  8),
    ('SR',  'erp',         'organization', 'unspecified', 'planned',  9),
    ('OP',  'erp',         'organization', 'unspecified', 'planned', 10),
    ('SV',  'erp',         'organization', 'unspecified', 'planned', 11),
    ('LG',  'erp',         'organization', 'unspecified', 'planned', 12),
    ('PR',  'erp',         'organization', 'unspecified', 'planned', 13),
    ('CA',  'erp',         'organization', 'unspecified', 'planned', 14),
    ('AU',  'erp',         'organization', 'unspecified', 'planned', 15),
    ('HR',  'erp',         'organization', 'unspecified', 'planned', 16),
    ('TS',  'platform',    'platform',     'none',        'planned', 17),
    ('CC',  'platform',    'platform',     'none',        'planned', 18),
    ('SA',  'platform',    'platform',     'none',        'planned', 19);

-- System roles are neither deleted nor renamed; new codes fail the CHECK.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION system_roles_fixed() RETURNS trigger AS $$
BEGIN
    IF TG_LEVEL = 'STATEMENT' THEN
        RAISE EXCEPTION 'system roles cannot be truncated' USING ERRCODE = 'insufficient_privilege';
    END IF;
    IF TG_OP = 'UPDATE' AND NEW.code = OLD.code AND NEW.contour = OLD.contour AND NEW.scope = OLD.scope THEN
        RETURN NEW; -- status, branch rule and order change by migration
    END IF;
    RAISE EXCEPTION 'system role % cannot be deleted or renamed', COALESCE(OLD.code, '') USING ERRCODE = 'insufficient_privilege';
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER system_roles_fixed BEFORE UPDATE OR DELETE ON system_roles
    FOR EACH ROW EXECUTE FUNCTION system_roles_fixed();
CREATE TRIGGER system_roles_no_truncate BEFORE TRUNCATE ON system_roles
    FOR EACH STATEMENT EXECUTE FUNCTION system_roles_fixed();

-- +goose Down
DROP TABLE IF EXISTS system_roles;
DROP FUNCTION IF EXISTS system_roles_fixed();
