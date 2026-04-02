// Package store provides PostgreSQL-backed repository implementations
// for the gateway domain types.
package store

import (
	"context"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore holds a connection pool and provides repository methods.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore creates a new PostgresStore with the given connection pool.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

// Pool returns the underlying connection pool for use in transactions.
func (s *PostgresStore) Pool() *pgxpool.Pool {
	return s.pool
}

// RunMigrations applies all up-migration SQL files from the given filesystem in order.
// The filesystem should contain files at its root (e.g., "001_initial_schema.up.sql").
// Typically the caller embeds the gateway/migrations directory and passes it here.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool, migrationFS fs.FS) error {
	entries, err := fs.ReadDir(migrationFS, ".")
	if err != nil {
		return fmt.Errorf("reading migration directory: %w", err)
	}

	// Collect and sort up-migration files.
	var upFiles []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".up.sql") {
			upFiles = append(upFiles, e.Name())
		}
	}
	sort.Strings(upFiles)

	for _, name := range upFiles {
		data, err := fs.ReadFile(migrationFS, name)
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", name, err)
		}
		if _, err := pool.Exec(ctx, string(data)); err != nil {
			return fmt.Errorf("executing migration %s: %w", name, err)
		}
	}

	return nil
}
