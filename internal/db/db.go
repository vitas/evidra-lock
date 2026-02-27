// Package db provides Postgres connection pool initialisation and schema migration.
package db

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Connect creates a pgxpool, pings the server, and runs all pending migrations.
// Returns the pool ready for use. Caller must call pool.Close() when done.
func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("db.Connect: create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db.Connect: ping: %w", err)
	}

	if err := runMigrations(ctx, pool); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db.Connect: migrate: %w", err)
	}

	return pool, nil
}

// runMigrations executes all *.sql files from the embedded migrations directory
// in lexicographic order. Statements are separated by semicolons and executed
// individually so partial failures are visible.
func runMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire conn: %w", err)
	}
	defer conn.Release()

	for _, name := range names {
		data, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}

		for _, stmt := range splitStatements(string(data)) {
			if _, err := conn.Exec(ctx, stmt); err != nil {
				return fmt.Errorf("migration %s: %w", name, err)
			}
		}
		slog.Debug("migration applied", "file", name)
	}

	return nil
}

// splitStatements splits a SQL file on semicolons, skipping blank and comment-only lines.
func splitStatements(sql string) []string {
	var stmts []string
	for _, raw := range strings.Split(sql, ";") {
		s := strings.TrimSpace(raw)
		if s == "" || strings.HasPrefix(s, "--") {
			continue
		}
		// Strip inline comments (lines starting with --).
		var lines []string
		for _, line := range strings.Split(s, "\n") {
			if !strings.HasPrefix(strings.TrimSpace(line), "--") {
				lines = append(lines, line)
			}
		}
		s = strings.TrimSpace(strings.Join(lines, "\n"))
		if s != "" {
			stmts = append(stmts, s)
		}
	}
	return stmts
}
