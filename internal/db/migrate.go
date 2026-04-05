package db

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type migration struct {
	version int
	path    string
}

func Migrate(db *sql.DB) error {
	applied := make(map[int]bool)
	if err := ensureMigrationsTable(db); err != nil {
		return err
	}
	if err := loadAppliedMigrations(db, applied); err != nil {
		return err
	}
	tx, err := beginMigrationTx(db)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	migrations, err := readMigrations("migrations")
	if err != nil {
		return err
	}
	sortMigrations(migrations)
	for _, m := range migrations {
		if applied[m.version] {
			continue
		}
		query, err := readMigrationFile(m.path)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(query); err != nil {
			return fmt.Errorf("exec migration %d: %w", m.version, err)
		}
		if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES ($1)", m.version); err != nil {
			return fmt.Errorf("insert migration %d: %w", m.version, err)
		}
		slog.Info("applied migration",
			"version", m.version,
			"path", m.path,
		)
	}
	return tx.Commit()
}

func ensureMigrationsTable(db *sql.DB) error {
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS schema_migrations (version INT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now());"); err != nil {
		return fmt.Errorf("db query: %w", err)
	}
	return nil
}

func loadAppliedMigrations(db *sql.DB, applied map[int]bool) error {
	var version int

	rows, err := db.Query("SELECT version FROM schema_migrations;")
	if err != nil {
		return fmt.Errorf("db query: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		if err := rows.Scan(&version); err != nil {
			return fmt.Errorf("scan version: %w", err)
		}
		applied[version] = true
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("rows error: %w", err)
	}
	return nil
}

func beginMigrationTx(db *sql.DB) (*sql.Tx, error) {
	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	return tx, nil
}

func readMigrations(dir string) ([]migration, error) {
	var migrations []migration

	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}
	for _, file := range files {
		name := file.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		version, err := parseMigrationVersion(name)
		if err != nil {
			return nil, err
		}
		path := filepath.Join(dir, name)
		migrations = append(migrations, migration{
			version: version,
			path:    path,
		})
	}
	return migrations, nil
}

func parseMigrationVersion(name string) (int, error) {
	parts := strings.Split(name, "_")
	versionStr := parts[0]
	version, err := strconv.Atoi(versionStr)
	if err != nil {
		return 0, fmt.Errorf("reading file atoi: %w", err)
	}
	return version, nil
}

func sortMigrations(migrations []migration) {
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].version < migrations[j].version
	})
}

func readMigrationFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read migration file error: %w", err)
	}
	return string(data), nil
}
