# Update Job API - Example Test Case

## Scenario
Update the prompt of an existing job where the prompt is stored within the payload JSON.

## Initial Job State
```json
{
  "id": "123e4567-e89b-12d3-a456-426614174000",
  "name": "Test Job",
  "user_id": "user-123",
  "workspace_id": "workspace-456",
  "payload": "{\"prompt\":\"Original prompt content\",\"resource_name\":\"ai_agent\",\"resource_data\":\"agent-config-123\"}",
  "status": "created",
  "type": "scheduled",
  "schedule": "0 0 * * *",
  "interval": null,
  "is_deleted": false,
  "next_run_at": "2024-01-01T00:00:00Z",
  "last_run_at": null,
  "current_task_id": null,
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:00:00Z",
  "version": 1,
  "river_job_id": 123
}
```

## API Request
```bash
PUT /api/jobs/123e4567-e89b-12d3-a456-426614174000
Authorization: Bearer your-jwt-token
Content-Type: application/json

{
  "prompt": "Updated prompt content with new instructions"
}
```

## Expected Response
```json
{
  "id": "123e4567-e89b-12d3-a456-426614174000",
  "name": "Test Job",
  "user_id": "user-123",
  "workspace_id": "workspace-456",
  "payload": "{\"prompt\":\"Updated prompt content with new instructions\",\"resource_name\":\"ai_agent\",\"resource_data\":\"agent-config-123\"}",
  "status": "created",
  "type": "scheduled",
  "schedule": "0 0 * * *",
  "interval": null,
  "is_deleted": false,
  "next_run_at": "2024-01-01T00:00:00Z",
  "last_run_at": null,
  "current_task_id": null,
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T12:30:00Z",
  "version": 2,
  "river_job_id": 123
}
```

## Key Changes Observed
1. **Payload**: Only the `prompt` field within the JSON payload was updated
2. **Resource Data**: `resource_name` and `resource_data` remained unchanged
3. **Version**: Incremented from 1 to 2
4. **Updated At**: Timestamp updated to current time
5. **Other Fields**: All other job properties remained unchanged

## Validation Points
- ✅ Only prompt was updated in payload JSON
- ✅ Other payload fields preserved
- ✅ Job version incremented
- ✅ Updated timestamp changed
- ✅ All other job properties unchanged
- ✅ JSON structure maintained

## Error Cases

### 1. Job Not Found
```bash
PUT /api/jobs/non-existent-id
```
```json
{
  "error": "job not found or access denied"
}
```

### 2. Job Processing
```bash
PUT /api/jobs/123e4567-e89b-12d3-a456-426614174000
```
```json
{
  "error": "cannot update job while it is processing"
}
```

### 3. Running Tasks
```bash
PUT /api/jobs/123e4567-e89b-12d3-a456-426614174000
```
```json
{
  "error": "cannot update job while tasks are running"
}
```

### 4. Invalid Prompt
```bash
PUT /api/jobs/123e4567-e89b-12d3-a456-426614174000
Content-Type: application/json

{
  "prompt": ""
}
```
```json
{
  "error": "Key: 'UpdateJobRequest.Prompt' Error:Field validation for 'Prompt' failed on the 'required' tag"
}
```

## Database Impact
The API correctly handles the JSONB payload field by:
1. Parsing the existing JSON string
2. Updating only the prompt field
3. Preserving all other payload data
4. Marshaling back to JSON string
5. Updating the database record

This ensures data integrity while allowing prompt updates as requested.