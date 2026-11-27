package roles

import (
	"context"
	"database/sql"
)

type Repository struct{ db *sql.DB }

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

// The way each role is held is fixed here, next to the counts it explains.
// Directions count members of active companies with the direction.
const rolesSQL = `
	SELECT r.code, r.contour, r.scope, r.branch_scope, r.status,
		CASE r.code WHEN 'MRX' THEN 'platform_role' WHEN 'MP' THEN 'company_direction' WHEN 'SL' THEN 'company_direction' ELSE 'none' END,
		r.code = 'MRX',
		CASE r.code
			WHEN 'MRX' THEN (SELECT count(*) FROM platform_roles WHERE role = 'mrx')
			WHEN 'MP' THEN (SELECT count(DISTINCT m.user_id) FROM company_members m JOIN companies c ON c.id = m.company_id
				WHERE c.is_buyer AND c.status = 'active' AND NOT c.is_demo)
			WHEN 'SL' THEN (SELECT count(DISTINCT m.user_id) FROM company_members m JOIN companies c ON c.id = m.company_id
				WHERE c.is_supplier AND c.status = 'active' AND NOT c.is_demo)
			ELSE 0 END,
		CASE r.code
			WHEN 'MP' THEN (SELECT count(*) FROM companies WHERE is_buyer AND status = 'active' AND NOT is_demo)
			WHEN 'SL' THEN (SELECT count(*) FROM companies WHERE is_supplier AND status = 'active' AND NOT is_demo)
			ELSE 0 END
	FROM system_roles r ORDER BY r.sort_order`

const mechanismsSQL = `
	SELECT 'owner', count(*) FROM company_members WHERE role = 'owner'
	UNION ALL SELECT 'employee', count(*) FROM company_members WHERE role = 'employee'
	UNION ALL SELECT 'admin', count(*) FROM platform_roles WHERE role = 'admin'
	UNION ALL SELECT p, (SELECT count(*) FROM platform_permissions WHERE permission = p)
		FROM unnest(ARRAY['catalog.moderate', 'catalog.categories', 'catalog.brands']) AS p`

func (r *Repository) directory(ctx context.Context) (Directory, error) {
	d := Directory{Roles: []Role{}, Mechanisms: []Mechanism{}}
	rows, err := r.db.QueryContext(ctx, rolesSQL)
	if err != nil {
		return d, err
	}
	for rows.Next() {
		var x Role
		if err := rows.Scan(&x.Code, &x.Contour, &x.Scope, &x.BranchScope, &x.Status, &x.Via, &x.Assignable, &x.Holders, &x.Companies); err != nil {
			rows.Close()
			return d, err
		}
		d.Roles = append(d.Roles, x)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return d, err
	}
	rows, err = r.db.QueryContext(ctx, mechanismsSQL)
	if err != nil {
		return d, err
	}
	defer rows.Close()
	for rows.Next() {
		var m Mechanism
		if err := rows.Scan(&m.Key, &m.Holders); err != nil {
			return d, err
		}
		d.Mechanisms = append(d.Mechanisms, m)
	}
	return d, rows.Err()
}
