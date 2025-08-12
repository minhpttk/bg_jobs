package main

import (
	"context"
	"gin-gorm-river-app/config"
	"gin-gorm-river-app/services"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig()

	// Initialize database
	db, err := config.NewDatabase(cfg)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// Initialize services
	jobService := services.NewJobService(db)
	riverClient := services.GetRiverClientInstance(db)
	recoveryService := services.NewRecoveryService(jobService, riverClient)

	// Start River client
	if err := riverClient.Client.Start(context.Background()); err != nil {
		log.Fatal("Failed to start River client:", err)
	}

	// Run recovery on startup
	log.Println("Running recovery check on startup...")
	if err := recoveryService.RecoverIncompleteIntervals(context.Background()); err != nil {
		log.Printf("Recovery check failed: %v", err)
	} else {
		log.Println("Recovery check completed")
	}

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("Shutting down worker...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := riverClient.Client.Shutdown(ctx); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}

	log.Println("Worker shutdown complete")
}
