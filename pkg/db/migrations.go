package db

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql
var MigrationsFS embed.FS

func RunMigrations(database *sql.DB) error {
	driver, err := sqlite3.WithInstance(database, &sqlite3.Config{})
	if err != nil {
		return fmt.Errorf("could not create sqlite driver: %v", err)
	}

	// Use embedded migrations instead of file system
	d, err := iofs.New(MigrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("could not create iofs driver: %v", err)
	}

	m, err := migrate.NewWithInstance(
		"iofs", d,
		"sqlite3", driver,
	)
	if err != nil {
		return fmt.Errorf("could not create migrate instance: %v", err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("could not run migrations: %v", err)
	}

	return nil
}
