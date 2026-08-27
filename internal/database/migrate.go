package database

import (
	"context"
	"database/sql"
	"fmt"

	"observability-platform/migrations"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

const migrationDir = "."

func Open(dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open migration database: %w", err)
	}
	return db, nil
}

func Up(ctx context.Context, db *sql.DB) error {
	goose.SetBaseFS(migrations.Files)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set migration dialect: %w", err)
	}
	if err := goose.UpContext(ctx, db, migrationDir); err != nil {
		return fmt.Errorf("apply database migrations: %w", err)
	}
	return nil
}

func Status(ctx context.Context, db *sql.DB) error {
	goose.SetBaseFS(migrations.Files)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set migration dialect: %w", err)
	}
	if err := goose.StatusContext(ctx, db, migrationDir); err != nil {
		return fmt.Errorf("read migration status: %w", err)
	}
	return nil
}
