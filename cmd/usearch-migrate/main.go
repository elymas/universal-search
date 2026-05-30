// Package main is the usearch-migrate entrypoint: a thin, idempotent runner that
// applies the PostgreSQL schema migrations via the existing custom Go runner
// (internal/index/pg.EnsureSchema) — NOT golang-migrate.
//
// SPEC-DEPLOY-001 REQ-DEPLOY-003 / REQ-DEPLOY-006 / D4:
//   - The Helm pre-install,pre-upgrade hook Job runs this binary so the schema
//     exists before any application Deployment starts.
//   - EnsureSchema execs every forward *.sql in MigrationsDir lexicographically
//     and is idempotent (re-run = no-op + drift check). Down-migrations
//     (*.down.sql) are excluded at the runner level (D2).
//
// Configuration (env):
//   - DATABASE_URL   pgx DSN, e.g. postgresql://user:pass@host:5432/db (required)
//   - MIGRATIONS_DIR directory holding the *.sql files (default /migrations,
//     which is where deploy/Dockerfile.usearch-migrate COPYs them)
//   - MIGRATE_TIMEOUT connect+apply deadline (Go duration, default 120s)
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/elymas/universal-search/internal/index/pg"
)

const (
	defaultMigrationsDir = "/migrations"
	defaultTimeout       = 120 * time.Second
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(_ []string, stdout, stderr *os.File) int {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		fmt.Fprintln(stderr, "usearch-migrate: DATABASE_URL is required")
		return 2
	}

	migrationsDir := os.Getenv("MIGRATIONS_DIR")
	if migrationsDir == "" {
		migrationsDir = defaultMigrationsDir
	}

	timeout := defaultTimeout
	if v := os.Getenv("MIGRATE_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			fmt.Fprintf(stderr, "usearch-migrate: invalid MIGRATE_TIMEOUT %q: %v\n", v, err)
			return 2
		}
		timeout = d
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client, err := pg.NewClient(ctx, pg.Config{
		ConnString:    dsn,
		MigrationsDir: migrationsDir,
	})
	if err != nil {
		fmt.Fprintf(stderr, "usearch-migrate: connect: %v\n", err)
		return 1
	}
	defer client.Close()

	fmt.Fprintf(stdout, "usearch-migrate: applying migrations from %s\n", migrationsDir)
	if err := client.EnsureSchema(ctx); err != nil {
		fmt.Fprintf(stderr, "usearch-migrate: ensure schema: %v\n", err)
		return 1
	}

	fmt.Fprintln(stdout, "usearch-migrate: schema ensured (idempotent)")
	return 0
}
