package config

import (
	"database/sql"
	"os"
	"strconv"

	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Database struct {
	GORM *gorm.DB
	Pool *pgxpool.Pool
}

func NewDatabase() (*Database, error) {
	// Setup separate pgx pool for River worker (better performance)
	ctx := context.Background()

	databaseURL := os.Getenv("DATABASE_URL")

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}

	// Set max connections with default fallback
	maxConnStr := os.Getenv("MAX_DB_CONNECTION")
	if maxConnStr == "" {
		maxConnStr = "100"
	}
	if maxConns, err := strconv.Atoi(maxConnStr); err != nil {
		return nil, err
	} else {
		config.MaxConns = int32(maxConns)
	}

	// Set min connections with default fallback
	minConnStr := os.Getenv("MIN_DB_CONNECTION")
	if minConnStr == "" {
		minConnStr = "20"
	}
	if minConns, err := strconv.Atoi(minConnStr); err != nil {
		return nil, err
	} else {
		config.MinConns = int32(minConns)
	}

	pgxPool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}

	// Setup GORM with database/sql
	sqlDB, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, err
	}

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: sqlDB,
	}), &gorm.Config{})

	if err != nil {
		return nil, err
	}

	return &Database{
		GORM: gormDB,
		Pool: pgxPool,
	}, nil
}
