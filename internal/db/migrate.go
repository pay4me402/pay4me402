package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"sort"
	"strings"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

func Migrate(ctx context.Context, database *sql.DB) error {
	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	sort.Slice(entries, func(i int, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		query, err := migrationFiles.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}
		if _, err := database.ExecContext(ctx, string(query)); err != nil {
			return fmt.Errorf("run migration %s: %w", entry.Name(), err)
		}
	}
	if err := ensureColumn(ctx, database, "transactions", "status", "TEXT NOT NULL DEFAULT 'success'"); err != nil {
		return err
	}
	return nil
}

func ensureColumn(ctx context.Context, database *sql.DB, table string, column string, definition string) error {
	rows, err := database.QueryContext(ctx, "PRAGMA table_info("+table+")")
	if err != nil {
		return fmt.Errorf("read %s columns: %w", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue sql.NullString
		var primaryKey int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return fmt.Errorf("scan %s columns: %w", table, err)
		}
		if strings.EqualFold(name, column) {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate %s columns: %w", table, err)
	}
	if _, err := database.ExecContext(ctx, "ALTER TABLE "+table+" ADD COLUMN "+column+" "+definition); err != nil {
		return fmt.Errorf("add %s.%s column: %w", table, column, err)
	}
	return nil
}
