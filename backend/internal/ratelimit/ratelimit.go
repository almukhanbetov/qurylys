// Package ratelimit keeps fixed-window counters in PostgreSQL, so limits hold
// across restarts and several API instances.
package ratelimit

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/qurylys/qurylys/backend/internal/httpx"
)

type Rule struct {
	Key    string
	Max    int
	Window time.Duration
}

type Limiter struct{ db *sql.DB }

func New(db *sql.DB) *Limiter { return &Limiter{db: db} }

func hashKey(k string) string {
	s := sha256.Sum256([]byte(k))
	return hex.EncodeToString(s[:])
}

// Hit counts one attempt against every rule before the protected work runs, so
// parallel requests cannot slip past. Any exceeded rule yields a 429.
func (l *Limiter) Hit(ctx context.Context, rules ...Rule) error {
	for _, r := range rules {
		var count, left int
		err := l.db.QueryRowContext(ctx, `
			INSERT INTO rate_limits AS r (key, count, expires_at)
			VALUES ($1, 1, now() + make_interval(secs => $2))
			ON CONFLICT (key) DO UPDATE SET
				count = CASE WHEN r.expires_at <= now() THEN 1 ELSE r.count + 1 END,
				expires_at = CASE WHEN r.expires_at <= now() THEN EXCLUDED.expires_at ELSE r.expires_at END
			RETURNING count, ceil(extract(epoch FROM expires_at - now()))::int`,
			hashKey(r.Key), r.Window.Seconds()).Scan(&count, &left)
		if err != nil {
			return fmt.Errorf("rate limit: %w", err)
		}
		if count > r.Max {
			e := httpx.NewError(http.StatusTooManyRequests, "too_many_requests", "Too many attempts, try again later")
			e.RetryAfter = max(left, 1)
			return e
		}
	}
	return nil
}

// Reset clears counters after a legitimate success.
func (l *Limiter) Reset(ctx context.Context, keys ...string) error {
	for _, k := range keys {
		if _, err := l.db.ExecContext(ctx, `DELETE FROM rate_limits WHERE key = $1`, hashKey(k)); err != nil {
			return fmt.Errorf("rate limit reset: %w", err)
		}
	}
	return nil
}
