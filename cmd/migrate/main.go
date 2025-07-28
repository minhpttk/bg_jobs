package main

import (
	"context"
	"flag"
	"gin-gorm-river-app/config"
	"gin-gorm-river-app/migrations"
	"log"
	"os"

	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

func main() {
	// Parse command line flags
	action := flag.String("action", "up", "Migration action: up, down, or setup")
	flag.Parse()

	log.Printf("Starting migration with action: %s", *action)

	// Initialize database connection
	db, err := config.NewDatabase()
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// Get underlying sql.DB for connection management
	sqlDB, err := db.GORM.DB()
	if err != nil {
		log.Fatal("Failed to get underlying sql.DB:", err)
	}
	defer sqlDB.Close()

	// Ensure pgx pool is closed when done
	defer db.Pool.Close()

	switch *action {
	case "up", "setup":
		runMigrationsUp(db)
	case "down":
		runMigrationsDown(db)
	default:
		log.Printf("Unknown action: %s", *action)
		log.Println("Available actions: up, down, setup")
		os.Exit(1)
	}
}

func runMigrationsUp(db *config.Database) {
	// Run GORM auto-migrations and setup
	if err := migrations.SetupDatabase(db.GORM); err != nil {
		log.Fatal("Failed to setup database:", err)
	}
	log.Println("GORM database setup completed successfully!")

	// Run River migrations
	if err := runRiverMigrations(db, rivermigrate.DirectionUp); err != nil {
		log.Fatal("Failed to run River migrations:", err)
	}
	log.Println("River migrations completed successfully!")
}

func runMigrationsDown(db *config.Database) {
	log.Println("Warning: GORM doesn't support automatic rollback.")
	log.Println("To rollback GORM migrations, use the SQL migration script:")
	log.Println("./scripts/run-migration.sh down")

	// Run River migrations down
	if err := runRiverMigrations(db, rivermigrate.DirectionDown); err != nil {
		log.Fatal("Failed to rollback River migrations:", err)
	}
	log.Println("River migrations rolled back successfully!")

	os.Exit(1)
}

func runRiverMigrations(db *config.Database, direction rivermigrate.Direction) error {
	riverDriver := riverpgxv5.New(db.Pool)
	migrator, err := rivermigrate.New(riverDriver, nil)
	if err != nil {
		return err
	}

	ctx := context.Background()
	_, err = migrator.Migrate(ctx, direction, &rivermigrate.MigrateOpts{})
	return err
}
