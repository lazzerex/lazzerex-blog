package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

//go:embed migrations/*.sql migrations/postgres/*.sql
var migrationFiles embed.FS

func ApplyMigrations(ctx context.Context, dbConn *sql.DB, dialect string) error {
	normalizedDialect := normalizeDialect(dialect)

	if _, err := dbConn.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`); err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	migrationsDir := migrationDirectory(normalizedDialect)

	entries, err := fs.ReadDir(migrationFiles, migrationsDir)
	if err != nil {
		return fmt.Errorf("read %s migrations directory: %w", normalizedDialect, err)
	}

	sort.Slice(entries, func(left, right int) bool {
		return entries[left].Name() < entries[right].Name()
	})

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}

		version := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		applied, err := migrationApplied(ctx, dbConn, version, normalizedDialect)
		if err != nil {
			return err
		}

		if applied {
			continue
		}

		migrationPath := path.Join(migrationsDir, entry.Name())
		sqlBytes, err := migrationFiles.ReadFile(migrationPath)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", migrationPath, err)
		}

		tx, err := dbConn.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("start migration transaction %s: %w", version, err)
		}

		if _, err := tx.ExecContext(ctx, string(sqlBytes)); err != nil {
			tx.Rollback()
			return fmt.Errorf("execute migration %s: %w", version, err)
		}

		if _, err := tx.ExecContext(ctx, bindPositionalArgs(`INSERT INTO schema_migrations (version) VALUES (?)`, normalizedDialect), version); err != nil {
			tx.Rollback()
			return fmt.Errorf("record migration %s: %w", version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", version, err)
		}
	}

	return nil
}

func migrationApplied(ctx context.Context, dbConn *sql.DB, version, dialect string) (bool, error) {
	var count int
	if err := dbConn.QueryRowContext(ctx, bindPositionalArgs(`SELECT COUNT(1) FROM schema_migrations WHERE version = ?`, dialect), version).Scan(&count); err != nil {
		return false, fmt.Errorf("check migration %s state: %w", version, err)
	}

	return count > 0, nil
}

func migrationDirectory(dialect string) string {
	if dialect == "postgres" {
		return path.Join("migrations", "postgres")
	}

	return "migrations"
}

func normalizeDialect(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "postgres" {
		return "postgres"
	}

	return "sqlite"
}

func bindPositionalArgs(query, dialect string) string {
	if dialect != "postgres" {
		return query
	}

	var builder strings.Builder
	builder.Grow(len(query) + 8)

	placeholder := 0
	for i := 0; i < len(query); i++ {
		if query[i] == '?' {
			placeholder++
			builder.WriteByte('$')
			builder.WriteString(strconv.Itoa(placeholder))
			continue
		}

		builder.WriteByte(query[i])
	}

	return builder.String()
}
