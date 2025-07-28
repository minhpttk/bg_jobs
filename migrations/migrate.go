package migrations

import (
	"gin-gorm-river-app/models"
	"log"

	"gorm.io/gorm"
)

// RunAutoMigrations runs GORM auto-migrations for all models
func RunAutoMigrations(db *gorm.DB) error {
	log.Println("Running GORM auto-migrations...")

	// Auto-migrate all models
	err := db.AutoMigrate(
		&models.Jobs{},
		&models.Tasks{},
	)

	if err != nil {
		log.Printf("Failed to run auto-migrations: %v", err)
		return err
	}

	log.Println("GORM auto-migrations completed successfully!")
	return nil
}

// SetupDatabase runs all necessary database setup
func SetupDatabase(db *gorm.DB) error {
	// Run auto-migrations
	if err := RunAutoMigrations(db); err != nil {
		return err
	}

	return nil
}
