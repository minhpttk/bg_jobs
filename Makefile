# Makefile for Background Job Service

.PHONY: help migrate-up migrate-down test-task-recovery build-api build-worker run-api run-worker

# Default target
help:
	@echo "Available commands:"
	@echo "  migrate-up                    - Run database migrations"
	@echo "  migrate-down                  - Rollback database migrations"
	@echo "  migrate-remove-current-task-id - Remove current_task_id column"
	@echo "  test-task-recovery            - Test task recovery functionality"
	@echo "  test-auto-recovery            - Run automated task recovery test"
	@echo "  test-mock-recovery            - Run mock task recovery test (no DB required)"
	@echo "  build-api                     - Build API server"
	@echo "  build-worker                  - Build worker"
	@echo "  run-api                       - Run API server"
	@echo "  run-worker                    - Run worker"

# Database migrations
migrate-up:
	@echo "Running database migrations..."
	go run cmd/migrate/main.go -action=setup

migrate-down:
	@echo "Rolling back database migrations..."
	go run cmd/migrate/main.go -action=down

# Remove current_task_id column migration
migrate-remove-current-task-id:
	@echo "Removing current_task_id column from jobs table..."
	psql $(DATABASE_URL) -f migrations/003_remove_current_task_id_from_jobs.sql

# Build applications
build-api:
	@echo "Building API server..."
	go build -o bin/api.exe ./cmd/api

build-worker:
	@echo "Building worker..."
	go build -o bin/worker.exe ./cmd/worker

# Run applications
run-api:
	@echo "Starting API server..."
	go run cmd/api/main.go

run-worker:
	@echo "Starting worker..."
	go run cmd/worker/main.go

# Test task recovery
test-task-recovery:
	@echo "Testing Task Recovery Functionality"
	@echo "==================================="
	@echo "1. Running migrations..."
	@$(MAKE) migrate-up
	@echo "2. Starting worker for testing..."
	@echo "   (This will start the worker and you can manually test task recovery)"
	@echo "   Press Ctrl+C to stop the worker"
	@$(MAKE) run-worker

# Automated task recovery test
test-auto-recovery:
	@echo "Running Automated Task Recovery Test"
	@echo "===================================="
	@echo "Building and running test script..."
	@go build -o bin/test_task_recovery.exe scripts/test_task_recovery_simple.go
	@bin\test_task_recovery.exe
	@echo "Cleaning up..."
	@if exist bin\test_task_recovery.exe del bin\test_task_recovery.exe

# Mock task recovery test (no database required)
test-mock-recovery:
	@echo "Running Mock Task Recovery Test"
	@echo "==============================="
	@echo "Running mock test (no database required)..."
	@go run scripts/test_task_recovery_mock.go