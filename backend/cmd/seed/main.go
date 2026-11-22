// Command seed loads clearly labelled demo data. It is idempotent: running it
// again updates the same rows instead of creating duplicates.
//
// It never runs automatically. With APP_ENV=production it refuses to run
// unless -allow-production is passed explicitly.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/qurylys/qurylys/backend/internal/seed"
)

func main() {
	allowProd := flag.Bool("allow-production", false, "allow loading demo data when APP_ENV=production")
	flag.Parse()

	if err := run(*allowProd); err != nil {
		fmt.Fprintln(os.Stderr, "seed:", err)
		os.Exit(1)
	}
	fmt.Println("seed: demo data is up to date")
}

func run(allowProd bool) error {
	if os.Getenv("APP_ENV") == "production" && !allowProd {
		return fmt.Errorf("refusing to load demo data with APP_ENV=production (pass -allow-production to override)")
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

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	return seed.LoadDemo(ctx, db)
}
