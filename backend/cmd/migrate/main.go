// Command migrate applies the embedded Goose SQL migrations.
//
//	migrate up | down | status | version | redo | reset
//
// It reads DATABASE_URL from the environment. Schema-changing commands hold a
// PostgreSQL advisory lock, so parallel runs are serialised; a run that cannot
// get the lock within MIGRATE_LOCK_WAIT (default 2m) fails with a non-zero exit.
// "reset" (roll back everything) is refused when APP_ENV=production.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/qurylys/qurylys/backend/migrations"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: migrate up|down|status|version|redo|reset")
	}
	command := args[0]
	switch command {
	case "up", "down", "status", "version", "redo", "reset":
	default:
		return fmt.Errorf("unknown command %q", command)
	}
	if command == "reset" && os.Getenv("APP_ENV") == "production" {
		return fmt.Errorf("reset is disabled when APP_ENV=production")
	}

	lockWait := 2 * time.Minute
	if v := os.Getenv("MIGRATE_LOCK_WAIT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			return fmt.Errorf("MIGRATE_LOCK_WAIT must be a positive duration like 2m, got %q", v)
		}
		lockWait = d
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), lockWait+10*time.Minute)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	p, err := migrations.NewProvider(db, lockWait)
	if err != nil {
		return err
	}

	switch command {
	case "up":
		res, err := p.Up(ctx)
		printResults(res)
		if err == nil && len(res) == 0 {
			fmt.Println("no pending migrations")
		}
		return err
	case "down":
		res, err := p.Down(ctx)
		printResults([]*goose.MigrationResult{res})
		return err
	case "redo":
		res, err := p.Down(ctx)
		printResults([]*goose.MigrationResult{res})
		if err != nil {
			return err
		}
		up, err := p.UpByOne(ctx)
		printResults([]*goose.MigrationResult{up})
		return err
	case "reset":
		res, err := p.DownTo(ctx, 0)
		printResults(res)
		return err
	case "status":
		st, err := p.Status(ctx)
		if err != nil {
			return err
		}
		for _, s := range st {
			applied := "pending"
			if s.State == goose.StateApplied {
				applied = s.AppliedAt.UTC().Format(time.RFC3339)
			}
			fmt.Printf("%-24s %s\n", applied, s.Source.Path)
		}
		return nil
	case "version":
		v, err := p.GetDBVersion(ctx)
		if err != nil {
			return err
		}
		fmt.Println(v)
		return nil
	}
	return nil
}

func printResults(res []*goose.MigrationResult) {
	for _, r := range res {
		if r != nil {
			fmt.Println(r.String())
		}
	}
}
