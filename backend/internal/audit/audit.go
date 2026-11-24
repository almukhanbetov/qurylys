// Package audit appends security-relevant events. Never put codes, passwords or tokens in details.
package audit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/qurylys/qurylys/backend/internal/database"
)

type Event struct {
	ActorID, CompanyID, TargetUserID int64 // 0 = none
	Action                           string
	Details                          map[string]any
}

// Write must run in the same transaction as the change it describes.
func Write(ctx context.Context, q database.Querier, e Event) error {
	if e.Details == nil {
		e.Details = map[string]any{}
	}
	d, err := json.Marshal(e.Details)
	if err != nil {
		return err
	}
	if _, err := q.ExecContext(ctx, `
		INSERT INTO audit_events (actor_user_id, company_id, target_user_id, action, details)
		VALUES (NULLIF($1::bigint, 0), NULLIF($2::bigint, 0), NULLIF($3::bigint, 0), $4, $5::jsonb)`,
		e.ActorID, e.CompanyID, e.TargetUserID, e.Action, string(d)); err != nil {
		return fmt.Errorf("audit: %w", err)
	}
	return nil
}
