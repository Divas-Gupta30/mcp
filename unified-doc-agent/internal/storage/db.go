package storage

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

var DB *pgxpool.Pool

func InitDB() error {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = "postgres://dg_project:password@localhost:5432/unified_agent"
	}

	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		return fmt.Errorf("failed to connect to Postgres: %w", err)
	}

	DB = pool
	return nil
}
