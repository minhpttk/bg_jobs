#!/bin/bash

# Test script for task recovery mechanism
# This script demonstrates how the recovery system works

set -e

echo "🧪 Testing Task Recovery Mechanism"
echo "=================================="

# Configuration
API_URL="http://localhost:8080"
DATABASE_URL="${DATABASE_URL:-postgres://postgres:postgres@localhost:5432/bg_jobs?sslmode=disable}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Helper functions
log_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

log_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

log_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

log_error() {
    echo -e "${RED}❌ $1${NC}"
}

# Check if services are running
check_services() {
    log_info "Checking if services are running..."
    
    if ! curl -s "$API_URL/health" > /dev/null; then
        log_error "API server is not running on $API_URL"
        exit 1
    fi
    
    log_success "API server is running"
}

# Create a test job
create_test_job() {
    log_info "Creating test interval job..."
    
    JOB_RESPONSE=$(curl -s -X POST "$API_URL/jobs" \
        -H "Content-Type: application/json" \
        -d '{
            "name": "Test Recovery Job",
            "workspace_id": "123e4567-e89b-12d3-a456-426614174000",
            "payload": "{\"prompt\": \"Test task\", \"resource_name\": \"ai_agent\", \"resource_data\": \"{\\\"name\\\": \\\"Test Agent\\\", \\\"url\\\": \\\"http://localhost:3000\\\"}\"}",
            "type": "interval",
            "interval": "{\"interval_type\": \"minutes\", \"value\": \"*/1 * * * *\"}"
        }')
    
    JOB_ID=$(echo "$JOB_RESPONSE" | jq -r '.job_id')
    
    if [ "$JOB_ID" = "null" ] || [ -z "$JOB_ID" ]; then
        log_error "Failed to create job: $JOB_RESPONSE"
        exit 1
    fi
    
    log_success "Created test job with ID: $JOB_ID"
    echo "$JOB_ID"
}

# Wait for job to start processing
wait_for_job_processing() {
    local job_id=$1
    local max_attempts=30
    local attempt=1
    
    log_info "Waiting for job to start processing..."
    
    while [ $attempt -le $max_attempts ]; do
        JOB_STATUS=$(curl -s "$API_URL/jobs/$job_id" | jq -r '.status')
        
        if [ "$JOB_STATUS" = "active" ]; then
            log_success "Job is active and processing"
            return 0
        fi
        
        log_info "Attempt $attempt/$max_attempts: Job status is $JOB_STATUS"
        sleep 2
        attempt=$((attempt + 1))
    done
    
    log_error "Job did not start processing within expected time"
    return 1
}

# Check job progress
check_job_progress() {
    local job_id=$1
    
    log_info "Checking job progress..."
    
    JOB_DATA=$(curl -s "$API_URL/jobs/$job_id")
    INTERVAL_PROGRESS=$(echo "$JOB_DATA" | jq -r '.interval_progress')
    
    if [ "$INTERVAL_PROGRESS" != "null" ] && [ -n "$INTERVAL_PROGRESS" ]; then
        log_success "Job has progress tracking enabled"
        echo "$INTERVAL_PROGRESS" | jq '.'
    else
        log_warning "No progress data found for job"
    fi
}

# Simulate server restart (stop worker)
simulate_restart() {
    log_warning "Simulating server restart..."
    log_info "You can now stop the worker process (Ctrl+C) to simulate a restart"
    log_info "After stopping, restart the worker to see recovery in action"
    
    read -p "Press Enter when you've stopped the worker..."
}

# Check for incomplete intervals
check_incomplete_intervals() {
    log_info "Checking for incomplete intervals in database..."
    
    # This would typically be done through the API or direct database query
    # For demo purposes, we'll show the expected behavior
    
    log_info "Expected behavior:"
    echo "1. Worker startup should detect incomplete intervals"
    echo "2. Recovery jobs should be scheduled automatically"
    echo "3. Incomplete tasks should be resumed"
    echo "4. Progress should be maintained"
}

# Main test flow
main() {
    echo "Starting task recovery test..."
    echo
    
    # Check prerequisites
    check_services
    
    # Create test job
    JOB_ID=$(create_test_job)
    
    # Wait for processing to start
    if wait_for_job_processing "$JOB_ID"; then
        # Check initial progress
        check_job_progress "$JOB_ID"
        
        # Simulate restart scenario
        simulate_restart
        
        # Show what should happen
        check_incomplete_intervals
        
        log_success "Test setup complete!"
        echo
        echo "Next steps:"
        echo "1. Restart the worker process"
        echo "2. Watch the logs for recovery messages"
        echo "3. Check that incomplete tasks are resumed"
        echo "4. Verify that progress is maintained"
    else
        log_error "Test failed: Job did not start processing"
        exit 1
    fi
}

# Run the test
main "$@"