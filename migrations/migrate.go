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

// ✅ ADD: Run SQL migrations
func RunSQLMigrations(db *gorm.DB) error {
	log.Println("Running SQL migrations...")

	// Migration 002: Add current_task_id to jobs table
	migration002 := `
		-- Add current_task_id column to jobs table for task recovery
		ALTER TABLE jobs ADD COLUMN IF NOT EXISTS current_task_id UUID;

		-- Add index for better performance when querying by current_task_id
		CREATE INDEX IF NOT EXISTS idx_jobs_current_task_id ON jobs (current_task_id) WHERE current_task_id IS NOT NULL;
	`

	if err := db.Exec(migration002).Error; err != nil {
		log.Printf("Failed to run migration 002: %v", err)
		return err
	}

	log.Println("SQL migrations completed successfully!")
	return nil
}

// SetupDatabase runs all necessary database setup
func SetupDatabase(db *gorm.DB) error {
	// Run auto-migrations
	if err := RunAutoMigrations(db); err != nil {
		return err
	}

	// ✅ ADD: Run SQL migrations
	if err := RunSQLMigrations(db); err != nil {
		return err
	}

	return nil
}
