# Task Recovery Test Scripts

This directory contains automated test scripts for testing the new task recovery mechanism.

## Files

- `test_task_recovery.go` - Main test script (Go)
- `test_config.go` - Test configuration
- `test_task_recovery.ps1` - PowerShell script for Windows
- `test_task_recovery.bat` - Batch script for Windows
- `README.md` - This file

## Quick Start

### Using Makefile (Recommended)

```bash
# Run automated test
make test-auto-recovery
```

### Manual Execution

#### Windows (PowerShell)
```powershell
.\scripts\test_task_recovery.ps1
```

#### Windows (Command Prompt)
```cmd
scripts\test_task_recovery.bat
```

#### Linux/Mac
```bash
# Build and run directly
go build -o bin/test_task_recovery ./scripts/test_task_recovery.go
./bin/test_task_recovery
```

## Test Scenario

The test script performs the following steps:

1. **Create Test Job** - Creates a job in the database
2. **Create Running Task** - Creates a task with "running" status (simulating interrupted execution)
3. **Verify Task Status** - Confirms the task exists and is running
4. **Run Recovery** - Executes the task recovery process (simulating server restart)
5. **Check Recovery Jobs** - Verifies recovery jobs were added to River queue
6. **Test Job Processing** - Tests the recovery job processing
7. **Wait for Processing** - Waits for job to be processed
8. **Verify Results** - Checks that the task was processed correctly
9. **Cleanup** - Removes test data (configurable)

## Configuration

You can customize the test by modifying `test_config.go`:

```go
func DefaultTestConfig() *TestConfig {
    return &TestConfig{
        TestJobName:     "Test Recovery Job",
        TestPrompt:      "Test recovery scenario - automated test",
        TestAgentName:   "test_agent",
        TestAgentURL:    "http://localhost:8080",
        WaitTimeSeconds: 2,
        AutoCleanup:     true,
    }
}
```

### Configuration Options

- `TestJobName` - Name of the test job
- `TestPrompt` - Prompt for the AI agent
- `TestAgentName` - Name of the test agent
- `TestAgentURL` - URL of the test agent
- `WaitTimeSeconds` - How long to wait for job processing
- `AutoCleanup` - Whether to automatically clean up test data

## Prerequisites

1. **Database** - PostgreSQL database must be running and accessible
2. **Environment** - `.env` file with database configuration
3. **Go** - Go 1.19+ installed
4. **Dependencies** - All Go dependencies installed (`go mod tidy`)

## Environment Variables

Make sure your `.env` file contains:

```env
DATABASE_URL=postgres://username:password@localhost:5432/database_name
```

## Expected Output

Successful test run should show:

```
🚀 Starting Task Recovery Test...
✅ Go found
✅ Test script built successfully
🧪 Running Task Recovery Test...
Step 1: Creating test job...
✅ Created test job: [job-id]
Step 2: Creating task and setting to running...
✅ Created test task: [task-id] (status: running)
Step 3: Verifying task status...
✅ Found 1 running tasks in database
Step 4: Running task recovery (simulating server restart)...
✅ Task recovery completed
Step 5: Checking River queue for recovery jobs...
✅ Task still exists in database: [task-id] (status: running)
Step 6: Testing recovery job processing...
✅ Added recovery job to River queue: [river-job-id]
Step 7: Waiting for job processing...
Step 8: Verifying task processing...
✅ Task processed: [task-id] (status: completed)
Step 9: Cleaning up test data...
✅ Test data cleaned up
✅ Test completed successfully!
🎉 Task Recovery Test completed!
```

## Troubleshooting

### Common Issues

1. **Database Connection Failed**
   - Check your `.env` file and database connection
   - Ensure PostgreSQL is running

2. **Go Build Failed**
   - Run `go mod tidy` to install dependencies
   - Check Go version (requires 1.19+)

3. **River Client Failed**
   - Ensure River is properly configured
   - Check database schema

4. **Test Data Not Cleaned Up**
   - Check database permissions
   - Manual cleanup may be required

### Debug Mode

To run with more verbose output, modify the test script to add more logging:

```go
log.SetLevel(log.DebugLevel)
```

## Manual Testing

If you want to test manually without the automated script:

1. Start the worker: `make run-worker`
2. Create a job through the API
3. Kill the worker process while a task is running
4. Restart the worker: `make run-worker`
5. Check logs for recovery messages

## Integration with CI/CD

The test script can be integrated into CI/CD pipelines:

```yaml
# Example GitHub Actions step
- name: Test Task Recovery
  run: |
    make test-auto-recovery
```