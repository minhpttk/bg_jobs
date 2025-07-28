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

	maxConnections := os.Getenv("MAX_DB_CONNECTION")
	if maxConnections == "" {
		maxConnections = "100"
	}

	minConnections := os.Getenv("MIN_DB_CONNECTION")
	if minConnections == "" {
		minConnections = "20"
	}

	maxConns, err := strconv.Atoi(maxConnections)
	if err != nil {
		return nil, err
	}
	config.MaxConns = int32(maxConns)

	minConns, err := strconv.Atoi(minConnections)
	if err != nil {
		return nil, err
	}
	config.MinConns = int32(minConns)

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
