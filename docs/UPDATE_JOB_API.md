# Update Job API

## Overview
This API endpoint allows updating the prompt of an existing job. Only the prompt field can be updated - all other job properties remain unchanged.

## Endpoint
```
PUT /api/jobs/{id}
```

## Authentication
Requires JWT authentication. Include the JWT token in the Authorization header:
```
Authorization: Bearer <your-jwt-token>
```

## Request Body
```json
{
  "prompt": "Updated prompt content here"
}
```

### Request Parameters
- `prompt` (string, required): The new prompt content. Maximum length: 20,000 characters.

## Response

### Success Response (200 OK)
Returns the updated job object:
```json
{
  "id": "uuid",
  "name": "Job Name",
  "user_id": "uuid",
  "workspace_id": "uuid",
  "payload": "{\"prompt\":\"Updated prompt content here\",\"resource_name\":\"ai_agent\",\"resource_data\":\"...\"}",
  "status": "active",
  "type": "scheduled",
  "schedule": "0 0 * * *",
  "interval": null,
  "is_deleted": false,
  "next_run_at": "2024-01-01T00:00:00Z",
  "last_run_at": null,
  "current_task_id": null,
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:00:00Z",
  "version": 2,
  "river_job_id": 123
}
```

### Error Responses

#### 400 Bad Request
```json
{
  "error": "Job ID is required"
}
```
or
```json
{
  "error": "Key: 'UpdateJobRequest.Prompt' Error:Field validation for 'Prompt' failed on the 'required' tag"
}
```

#### 401 Unauthorized
```json
{
  "error": "Unauthorized"
}
```

#### 500 Internal Server Error
```json
{
  "error": "job not found or access denied"
}
```
or
```json
{
  "error": "cannot update job while it is processing"
}
```
or
```json
{
  "error": "cannot update job while tasks are running"
}
```

## Business Rules
1. **Ownership**: Only the job owner can update their jobs
2. **Job Status**: Jobs with status "processing" cannot be updated
3. **Running Tasks**: Jobs with running tasks cannot be updated
4. **Prompt Only**: Only the prompt field within the payload JSON can be updated; all other fields remain unchanged
5. **Version Increment**: The job version is automatically incremented on update
6. **Timestamp Update**: The `updated_at` timestamp is automatically updated

## Example Usage

### cURL
```bash
curl -X PUT \
  http://localhost:3008/api/jobs/123e4567-e89b-12d3-a456-426614174000 \
  -H 'Authorization: Bearer your-jwt-token' \
  -H 'Content-Type: application/json' \
  -d '{
    "prompt": "Updated prompt for the job"
  }'
```

### JavaScript/Fetch
```javascript
const response = await fetch('/api/jobs/123e4567-e89b-12d3-a456-426614174000', {
  method: 'PUT',
  headers: {
    'Authorization': 'Bearer your-jwt-token',
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({
    prompt: 'Updated prompt for the job'
  })
});

const updatedJob = await response.json();
```

## Notes
- The API preserves the existing payload structure and only updates the prompt field
- The job's version number is incremented to track changes
- If the job has running tasks, the update will be rejected to prevent data inconsistency
- The endpoint uses PUT method as it's updating an existing resource