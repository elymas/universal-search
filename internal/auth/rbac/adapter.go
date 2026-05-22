package rbac

import (
	"fmt"

	pgadapter "github.com/casbin/casbin-pg-adapter"
	"github.com/go-pg/pg/v10"
)

// PGAdapter wraps casbin-pg-adapter with an isolated *pg.DB connection.
// NFR-AUTH2-004: Policy storage connection isolated from hot-path pgxpool.
type PGAdapter struct {
	adapter *pgadapter.Adapter
	db      *pg.DB
}

// NewPGAdapter creates a new Casbin PG adapter with an isolated database connection.
// The DSN is used to create a separate *pg.DB instance that is not shared with
// the application's hot-path pgxpool.
func NewPGAdapter(dsn string) (*PGAdapter, error) {
	if dsn == "" {
		return nil, fmt.Errorf("rbac: pg_dsn is required when rbac is enabled")
	}

	opt, err := pg.ParseURL(dsn)
	if err != nil {
		return nil, fmt.Errorf("rbac: parse pg_dsn: %w", err)
	}

	db := pg.Connect(opt)

	adapter, err := pgadapter.NewAdapterByDB(db)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("rbac: create pg adapter: %w", err)
	}

	return &PGAdapter{adapter: adapter, db: db}, nil
}

// Close shuts down the isolated database connection.
func (a *PGAdapter) Close() error {
	if a.db != nil {
		return a.db.Close()
	}
	return nil
}

// Adapter returns the underlying casbin adapter interface.
func (a *PGAdapter) Adapter() *pgadapter.Adapter {
	return a.adapter
}

// DB returns the isolated *pg.DB for testing purposes.
func (a *PGAdapter) DB() *pg.DB {
	return a.db
}
