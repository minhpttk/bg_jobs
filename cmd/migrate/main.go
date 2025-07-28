package main

import (
	"flag"
	"gin-gorm-river-app/config"
	"gin-gorm-river-app/migrations"
	"log"
	"os"
)

func main() {
	// Parse command line flags
	var action = flag.String("action", "up", "Migration action: up, down, or setup")
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

	switch *action {
	case "up", "setup":
		// Run GORM auto-migrations and setup
		if err := migrations.SetupDatabase(db.GORM); err != nil {
			log.Fatal("Failed to setup database:", err)
		}
		log.Println("Database setup completed successfully!")

	case "down":
		log.Println("Warning: GORM doesn't support automatic rollback.")
		log.Println("To rollback, use the SQL migration script:")
		log.Println("./scripts/run-migration.sh down")
		os.Exit(1)

	default:
		log.Printf("Unknown action: %s", *action)
		log.Println("Available actions: up, down, setup")
		os.Exit(1)
	}
}
