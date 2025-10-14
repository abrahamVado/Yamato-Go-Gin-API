package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"

	appmigrations "github.com/example/Yamato-Go-Gin-API/migrations"
)

// Migrator knows how to apply embedded SQL migrations against a database connection.
type Migrator struct {
	db *sql.DB
}

// NewMigrator prepares a migrator for the provided database connection.
func NewMigrator(db *sql.DB) (*Migrator, error) {
	//1.- Validate the incoming database handle to avoid nil pointer dereferences.
	if db == nil {
		return nil, errors.New("database connection is required")
	}

	//2.- Return the migrator so callers can execute the embedded SQL files in order.
	return &Migrator{db: db}, nil
}

// Apply runs every migration file in lexical order inside individual transactions.
func (m *Migrator) Apply(ctx context.Context) error {
	//1.- Prevent usage when the migrator has not been constructed correctly.
	if m == nil || m.db == nil {
		return errors.New("migrator is not initialized")
	}

	//2.- Build the ordered list of migration directories that must be processed.
	migrationDirs := []string{
		"0001_core",
		"0002_join_requests",
	}

	for _, migrationDir := range migrationDirs {
		//3.- Collect every SQL file in the directory and sort them for deterministic execution.
		entries, err := fs.ReadDir(appmigrations.Core, migrationDir)
		if err != nil {
			return fmt.Errorf("failed to read migration directory %s: %w", migrationDir, err)
		}

		fileNames := make([]string, 0, len(entries))
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			fileNames = append(fileNames, entry.Name())
		}
		sort.Strings(fileNames)

		//4.- Execute each migration inside its own transaction to keep changes atomic per file.
		for _, name := range fileNames {
			path := filepath.Join(migrationDir, name)
			contents, readErr := appmigrations.Core.ReadFile(path)
			if readErr != nil {
				return fmt.Errorf("failed to read migration %s: %w", name, readErr)
			}

			tx, txErr := m.db.BeginTx(ctx, nil)
			if txErr != nil {
				return fmt.Errorf("failed to open transaction for %s: %w", name, txErr)
			}

			if _, execErr := tx.ExecContext(ctx, string(contents)); execErr != nil {
				rollbackErr := tx.Rollback()
				if rollbackErr != nil {
					return fmt.Errorf("failed to apply migration %s: %v (rollback error: %w)", name, execErr, rollbackErr)
				}
				return fmt.Errorf("failed to apply migration %s: %w", name, execErr)
			}

			if commitErr := tx.Commit(); commitErr != nil {
				return fmt.Errorf("failed to commit migration %s: %w", name, commitErr)
			}
		}
	}

	//5.- Nothing failed so we can report success to the caller.
	return nil
}
