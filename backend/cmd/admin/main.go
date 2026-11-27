// Command admin manages platform administrators and checks mail settings. It is the only way to grant
// the administrator role; the API cannot do it.
//
//	admin create -contact <email|phone> -name "<full name>"   # password on stdin
//	admin grant  -contact <email|phone>
//	admin revoke -contact <email|phone>
//	admin catalog-report                                      # read-only: what stage 3 will affect
//	admin test-email -contact <email>                         # sends one test message over SMTP
//
// The password is read from standard input and never from arguments, the
// environment or the repository.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/qurylys/qurylys/backend/internal/auth"
	"github.com/qurylys/qurylys/backend/internal/catalog"
	"github.com/qurylys/qurylys/backend/internal/config"
	"github.com/qurylys/qurylys/backend/internal/database"
	"github.com/qurylys/qurylys/backend/internal/delivery"
	"github.com/qurylys/qurylys/backend/internal/validate"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "admin:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: admin create|grant|revoke|test-email -contact <email|phone> [-name <full name>] | catalog-report")
	}
	if args[0] == "catalog-report" {
		return catalogReport()
	}
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	contact := fs.String("contact", "", "email or phone")
	name := fs.String("name", "", "full name (create)")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	kind, value, err := validate.Contact(*contact)
	if err != nil {
		return fmt.Errorf("-contact: %w", err)
	}
	if args[0] == "test-email" {
		return testEmail(kind, value)
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	db, err := database.Open(ctx, dsn, database.PoolConfig{MaxOpenConns: 2})
	if err != nil {
		return err
	}
	defer db.Close()
	repo := auth.NewRepository(db)

	switch args[0] {
	case "create":
		full, err := validate.Text("full_name", *name, 120)
		if err != nil {
			return fmt.Errorf("-name: %w", err)
		}
		raw, err := io.ReadAll(io.LimitReader(os.Stdin, 4096))
		if err != nil {
			return err
		}
		password := strings.TrimRight(string(raw), "\r\n")
		if err := auth.CheckPassword(password, value); err != nil {
			return errors.New("password on stdin must be 10-128 characters and not a common one")
		}
		id, err := repo.CreateAdmin(ctx, full, kind, value, auth.HashPassword(password))
		if err != nil {
			return err
		}
		fmt.Printf("admin: created administrator, user id %d\n", id)
	case "grant", "revoke":
		if err := repo.SetAdmin(ctx, kind, value, args[0] == "grant"); err != nil {
			return err
		}
		fmt.Println("admin: done")
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
	return nil
}

func testEmail(kind, to string) error {
	if kind != validate.Email {
		return errors.New("-contact must be an email address")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.ContactDelivery != delivery.ModeSMTP {
		return fmt.Errorf("CONTACT_DELIVERY is %q, expected smtp", cfg.ContactDelivery)
	}
	m := delivery.Message{Channel: kind, To: to, Kind: delivery.KindTest, Locale: "ru"}
	if err := delivery.SendSMTP(context.Background(), cfg.SMTP, m); err != nil {
		return fmt.Errorf("not sent: %s", delivery.SafeReason(err))
	}
	fmt.Println("admin: the mail server accepted the message; check the inbox and spam")
	return nil
}

func catalogReport() error {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	db, err := database.Open(ctx, dsn, database.PoolConfig{MaxOpenConns: 1})
	if err != nil {
		return err
	}
	defer db.Close()
	return catalog.Report(ctx, db, os.Stdout)
}
