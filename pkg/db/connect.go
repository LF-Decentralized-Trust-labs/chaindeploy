package db

import (
	"database/sql"
	"fmt"
)

// OpenSQLite opens a SQLite database with recommended PRAGMAs for security and reliability.
// It enables foreign key enforcement, WAL journal mode, busy timeout, and sets
// appropriate connection pool limits for SQLite's single-writer model.
func OpenSQLite(dbPath string) (*sql.DB, error) {
	// Use connection string parameters for PRAGMAs that must be set per-connection.
	// _foreign_keys=on: enforce foreign key constraints (OFF by default in SQLite)
	// _journal_mode=WAL: write-ahead logging for better concurrency and crash recovery
	// _busy_timeout=5000: wait up to 5s on lock contention instead of returning SQLITE_BUSY
	// _synchronous=NORMAL: safe with WAL, avoids fsync on every commit
	dsn := fmt.Sprintf("%s?_foreign_keys=on&_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL", dbPath)

	database, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// SQLite supports only one writer at a time. Limit connections to avoid
	// contention and unnecessary SQLITE_BUSY errors.
	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)

	// Verify PRAGMAs were applied (some drivers ignore DSN params)
	var fkEnabled int
	if err := database.QueryRow("PRAGMA foreign_keys").Scan(&fkEnabled); err != nil {
		database.Close()
		return nil, fmt.Errorf("failed to check foreign_keys pragma: %w", err)
	}
	if fkEnabled != 1 {
		// Fall back to executing PRAGMAs directly
		if _, err := database.Exec("PRAGMA foreign_keys = ON"); err != nil {
			database.Close()
			return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
		}
	}

	return database, nil
}
