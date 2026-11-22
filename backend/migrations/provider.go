package migrations

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

// NewProvider returns a Goose provider over the embedded migrations that takes a
// PostgreSQL session-level advisory lock (pg_try_advisory_lock) for every
// operation that changes the schema. A second concurrent `migrate up` waits for
// the lock and fails after lockWait instead of applying migrations in parallel.
func NewProvider(db *sql.DB, lockWait time.Duration) (*goose.Provider, error) {
	const period = 2 // seconds between attempts
	attempts := uint64(lockWait / (period * time.Second))
	if attempts < 1 {
		attempts = 1
	}
	locker, err := lock.NewPostgresSessionLocker(
		lock.WithLockTimeout(period, attempts),
		lock.WithUnlockTimeout(period, 15),
	)
	if err != nil {
		return nil, fmt.Errorf("create migration lock: %w", err)
	}
	return goose.NewProvider(goose.DialectPostgres, db, FS,
		goose.WithSessionLocker(locker),
		goose.WithDisableGlobalRegistry(true),
	)
}
