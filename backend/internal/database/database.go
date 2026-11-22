// Package database opens the PostgreSQL connection pool via pgx's database/sql driver.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" driver
)

type PoolConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// Open creates a pool and verifies connectivity within the given context.
func Open(ctx context.Context, dsn string, pc PoolConfig) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(pc.MaxOpenConns)
	db.SetMaxIdleConns(pc.MaxIdleConns)
	db.SetConnMaxLifetime(pc.ConnMaxLifetime)
	db.SetConnMaxIdleTime(pc.ConnMaxIdleTime)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return db, nil
}
