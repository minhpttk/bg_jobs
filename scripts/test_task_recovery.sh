#!/bin/bash

# Test script for task recovery functionality
echo "Testing Task Recovery Functionality"
echo "==================================="

# Check if database is running
echo "1. Checking database connection..."
if ! pg_isready -h localhost -p 5432 -U postgres; then
    echo "❌ Database is not running. Please start the database first."
    exit 1
fi
echo "✅ Database is running"

# Run migrations
echo "2. Running migrations..."
cd /workspace
go run cmd/migrate/main.go -action=setup
if [ $? -ne 0 ]; then
    echo "❌ Migration failed"
    exit 1
fi
echo "✅ Migrations completed"

# Start worker in background
echo "3. Starting worker..."
go run cmd/worker/main.go &
WORKER_PID=$!

# Wait for worker to start
sleep 10

# Simulate server restart by killing worker
echo "4. Simulating server restart (killing worker)..."
kill $WORKER_PID
sleep 5

# Start worker again to test recovery
echo "5. Starting worker again to test recovery..."
go run cmd/worker/main.go &
NEW_WORKER_PID=$!

# Wait for recovery process
sleep 15

# Clean up
echo "6. Cleaning up..."
kill $NEW_WORKER_PID

echo "✅ Task recovery test completed!"
echo "Check the logs above to see if task recovery worked correctly."