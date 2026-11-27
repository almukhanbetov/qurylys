// Package roles is the read-only directory of the 19 roles of the
// specification and of how each is held today. It never assigns anything:
// MRX is granted on the user page (package auth), a company's directions and
// membership belong to the company (package companies), the other roles wait
// for their modules.
package roles

// How a role is held now. Keep in sync with the frontend dictionary.
const (
	ViaPlatformRole     = "platform_role"     // platform_roles, granted by an administrator
	ViaCompanyDirection = "company_direction" // membership in a company with this direction
	ViaNone             = "none"              // directory only, assignment not available
)

type Role struct {
	Code        string `json:"code"`
	Contour     string `json:"contour"`
	Scope       string `json:"scope"`
	BranchScope string `json:"branch_scope"`
	Status      string `json:"status"`
	Via         string `json:"via"`
	// Assignable: an administrator can assign it now (only MRX).
	Assignable bool `json:"assignable"`
	// Holders: users who hold it through Via; Companies: for directions.
	Holders   int `json:"holders"`
	Companies int `json:"companies"`
}

// Mechanism is current access that is not a role of the specification.
type Mechanism struct {
	Key     string `json:"key"` // owner, employee, admin, catalog.moderate, ...
	Holders int    `json:"holders"`
}

type Directory struct {
	Roles      []Role      `json:"roles"`
	Mechanisms []Mechanism `json:"mechanisms"`
}
