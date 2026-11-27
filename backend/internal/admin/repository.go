package admin

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/qurylys/qurylys/backend/internal/auth"
)

type Repository struct{ db *sql.DB }

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

func like(q string) string {
	return "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(q) + "%"
}

func split(s string) []string {
	if s == "" {
		return []string{}
	}
	return strings.Split(s, ",")
}

func pages(total int) int { return (total + pageSize - 1) / pageSize }

// Fixed conditions; the chosen key never reaches the SQL text.
var accessSQL = map[string]string{
	"staff": `(EXISTS (SELECT 1 FROM platform_roles r WHERE r.user_id = u.id) OR EXISTS (SELECT 1 FROM platform_permissions p WHERE p.user_id = u.id))`,
	"none":  `NOT EXISTS (SELECT 1 FROM platform_roles r WHERE r.user_id = u.id) AND NOT EXISTS (SELECT 1 FROM platform_permissions p WHERE p.user_id = u.id)`,
}

var membershipSQL = map[string]string{
	"with":    `EXISTS (SELECT 1 FROM company_members m WHERE m.user_id = u.id)`,
	"without": `NOT EXISTS (SELECT 1 FROM company_members m WHERE m.user_id = u.id)`,
}

func (r *Repository) users(ctx context.Context, f UserFilter) (Page[UserItem], error) {
	conds, args := []string{"TRUE"}, []any{}
	arg := func(v any) string { args = append(args, v); return "$" + strconv.Itoa(len(args)) }
	if f.Q != "" {
		ph := arg(like(f.Q))
		conds = append(conds, fmt.Sprintf("(u.full_name ILIKE %[1]s OR u.email ILIKE %[1]s OR u.phone ILIKE %[1]s)", ph))
	}
	switch {
	case f.Access == "":
	case accessSQL[f.Access] != "":
		conds = append(conds, accessSQL[f.Access])
	case f.Access == auth.RoleAdmin || f.Access == auth.RoleMRX:
		conds = append(conds, "EXISTS (SELECT 1 FROM platform_roles r WHERE r.user_id = u.id AND r.role = "+arg(f.Access)+")")
	default: // a grantable permission, checked by the handler
		conds = append(conds, "EXISTS (SELECT 1 FROM platform_permissions p WHERE p.user_id = u.id AND p.permission = "+arg(f.Access)+")")
	}
	if c := membershipSQL[f.Membership]; c != "" {
		conds = append(conds, c)
	}
	if f.Status != "" {
		conds = append(conds, "u.status = "+arg(f.Status))
	}
	where := " FROM users u WHERE " + strings.Join(conds, " AND ")

	out := Page[UserItem]{Items: []UserItem{}, Page: f.Page}
	if err := r.db.QueryRowContext(ctx, "SELECT count(*)"+where, args...).Scan(&out.Total); err != nil {
		return out, err
	}
	out.TotalPages = pages(out.Total)
	rows, err := r.db.QueryContext(ctx, `
		SELECT u.id, u.full_name, u.email, u.phone, u.status, u.created_at,
			COALESCE((SELECT string_agg(role, ',' ORDER BY role) FROM platform_roles pr WHERE pr.user_id = u.id), ''),
			COALESCE((SELECT string_agg(permission, ',' ORDER BY permission) FROM platform_permissions pp WHERE pp.user_id = u.id), ''),
			(SELECT count(*) FROM company_members m WHERE m.user_id = u.id)`+where+`
		ORDER BY u.id DESC LIMIT `+strconv.Itoa(pageSize)+` OFFSET `+arg((f.Page-1)*pageSize), args...)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var u UserItem
		var roles, perms string
		if err := rows.Scan(&u.ID, &u.FullName, &u.Email, &u.Phone, &u.Status, &u.CreatedAt, &roles, &perms, &u.CompanyCount); err != nil {
			return out, err
		}
		u.PlatformRoles, u.Permissions = split(roles), split(perms)
		out.Items = append(out.Items, u)
	}
	return out, rows.Err()
}

func (r *Repository) user(ctx context.Context, id int64) (*User, error) {
	u := User{Grants: []Grant{}, History: []GrantEvent{}, Memberships: []Membership{}}
	err := r.db.QueryRowContext(ctx, `
		SELECT id, full_name, email, email_verified_at, phone, phone_verified_at, status, created_at,
			(SELECT max(last_seen_at) FROM sessions s WHERE s.user_id = u.id)
		FROM users u WHERE id = $1`, id).Scan(
		&u.ID, &u.FullName, &u.Email, &u.EmailVerifiedAt, &u.Phone, &u.PhoneVerifiedAt, &u.Status, &u.CreatedAt, &u.LastSeenAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errNotFound
	}
	if err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT 'role', role, granted_at FROM platform_roles WHERE user_id = $1
		UNION ALL
		SELECT 'permission', permission, granted_at FROM platform_permissions WHERE user_id = $1
		ORDER BY 1 DESC, 2`, id)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var g Grant
		if err := rows.Scan(&g.Kind, &g.Value, &g.GrantedAt); err != nil {
			rows.Close()
			return nil, err
		}
		u.Grants = append(u.Grants, g)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = r.db.QueryContext(ctx, `
		SELECT a.action, COALESCE(a.details->>'role', a.details->>'permission', ''), COALESCE(actor.full_name, ''), a.created_at
		FROM audit_events a LEFT JOIN users actor ON actor.id = a.actor_user_id
		WHERE a.target_user_id = $1 AND a.action IN
			('platform_role.granted', 'platform_role.revoked', 'platform_permission.granted', 'platform_permission.revoked')
		ORDER BY a.id DESC LIMIT 50`, id)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var e GrantEvent
		if err := rows.Scan(&e.Action, &e.Value, &e.Actor, &e.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		u.History = append(u.History, e)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = r.db.QueryContext(ctx, `
		SELECT c.id, c.name, m.role, c.status, c.is_demo, v.status, m.created_at
		FROM company_members m
		JOIN companies c ON c.id = m.company_id
		JOIN company_verification v ON v.company_id = c.id
		WHERE m.user_id = $1 ORDER BY c.name, c.id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var m Membership
		if err := rows.Scan(&m.CompanyID, &m.Name, &m.Role, &m.Status, &m.IsDemo, &m.Verification, &m.JoinedAt); err != nil {
			return nil, err
		}
		u.Memberships = append(u.Memberships, m)
	}
	return &u, rows.Err()
}

const companyColumns = `c.id, c.name, c.bin, c.city, c.is_buyer, c.is_supplier, c.status, c.is_demo, v.status,
	(SELECT count(*) FROM company_members m WHERE m.company_id = c.id)`

const companyFrom = ` FROM companies c JOIN company_verification v ON v.company_id = c.id `

func scanCompany(s interface{ Scan(...any) error }, c *CompanyItem) error {
	return s.Scan(&c.ID, &c.Name, &c.BIN, &c.City, &c.IsBuyer, &c.IsSupplier, &c.Status, &c.IsDemo, &c.Verification, &c.MemberCount)
}

func (r *Repository) companies(ctx context.Context, q string, page int) (Page[CompanyItem], error) {
	where := `WHERE ($1 = '' OR c.name ILIKE $2 OR c.bin = $1)`
	out := Page[CompanyItem]{Items: []CompanyItem{}, Page: page}
	if err := r.db.QueryRowContext(ctx, `SELECT count(*)`+companyFrom+where, q, like(q)).Scan(&out.Total); err != nil {
		return out, err
	}
	out.TotalPages = pages(out.Total)
	rows, err := r.db.QueryContext(ctx, `SELECT `+companyColumns+companyFrom+where+`
		ORDER BY c.is_demo, c.name, c.id LIMIT `+strconv.Itoa(pageSize)+` OFFSET $3`, q, like(q), (page-1)*pageSize)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var c CompanyItem
		if err := scanCompany(rows, &c); err != nil {
			return out, err
		}
		out.Items = append(out.Items, c)
	}
	return out, rows.Err()
}

func (r *Repository) companyMembers(ctx context.Context, id int64) (*CompanyMembers, error) {
	out := CompanyMembers{Members: []Member{}}
	err := scanCompany(r.db.QueryRowContext(ctx, `SELECT `+companyColumns+companyFrom+`WHERE c.id = $1`, id), &out.Company)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errNotFound
	}
	if err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT u.id, u.full_name, m.role, m.created_at FROM company_members m JOIN users u ON u.id = m.user_id
		WHERE m.company_id = $1 ORDER BY m.role DESC, u.full_name, u.id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.UserID, &m.FullName, &m.Role, &m.JoinedAt); err != nil {
			return nil, err
		}
		out.Members = append(out.Members, m)
	}
	return &out, rows.Err()
}
