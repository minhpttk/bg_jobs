package main

import (
	"context"
	"gin-gorm-river-app/config"
	"gin-gorm-river-app/handlers"
	"gin-gorm-river-app/middleware"
	"gin-gorm-river-app/services"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		allowedOrigins := []string{
			"http://localhost:3000",
			"https://dev.hiagi.ai",
			"https://app.hiagi.ai",
		}
		origin := c.Request.Header.Get("Origin")
		for _, allowedOrigin := range allowedOrigins {
			if origin == allowedOrigin {
				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
				break
			}
		}

		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func NewRouter(server *gin.Engine, db *config.Database) {
	// Create River client for job handling
	riverClient := services.GetRiverClientInstance(db).Client

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := riverClient.Start(ctx); err != nil {
		log.Fatal("Failed to start River client: ", err)
	}

	// Default CORS configuration
	server.Use(CORSMiddleware())
	server.Use(gin.Logger())
	server.Use(gin.Recovery())

	router := server.Group("/api")

	jobService := services.NewJobService(db)
	jobHandler := handlers.NewJobHandler(jobService)

	// ===== PROTECTED:: job routings ====== //
	jobRouter := router.Group("/jobs", middleware.JWTAuthMiddleware())

	jobRouter.POST("", jobHandler.CreateJob)
	jobRouter.GET("", jobHandler.GetJobs)
	jobRouter.GET("/:id", jobHandler.GetJob)
	jobRouter.PATCH("/:id/pause", jobHandler.PauseJob)
	jobRouter.PATCH("/:id/resume", jobHandler.ResumeJob)
	jobRouter.DELETE("/:id", jobHandler.DeleteJob)
}

// Main function
func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: Error loading .env file:", err)
	}

	db, err := config.NewDatabase()
	if err != nil {
		log.Fatal("Could not connect to the database ", err)
	} else {
		log.Println("Connected to the database")
	}
	defer db.Pool.Close()

	server := gin.Default()
	NewRouter(server, db)

	port := os.Getenv("PORT")
	if port == "" {
		port = "3008"
	}

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: server,
	}

	// Graceful shutdown
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	log.Printf("API Server started on port %s", port)

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}
