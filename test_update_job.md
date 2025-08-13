# API Update Job

## Endpoint
```
PUT /api/jobs/:id
```

## Description
API này cho phép cập nhật job. Có 2 trường hợp:

1. **Chỉ cập nhật prompt/name**: Cập nhật trực tiếp job hiện tại
2. **Cập nhật time/target**: Tạo job mới và xóa job cũ

## Request Body
```json
{
  "name": "string (optional)",
  "payload": "string (optional)", 
  "type": "scheduled|interval (optional)",
  "schedule": "string (optional)",
  "interval": "string (optional)"
}
```

## Response
```json
{
  "job_id": "uuid",
  "is_new_job": "boolean",
  "message": "string"
}
```

## Ví dụ sử dụng

### 1. Chỉ cập nhật prompt (không tạo job mới)
```bash
curl -X PUT http://localhost:3008/api/jobs/123e4567-e89b-12d3-a456-426614174000 \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "payload": "New prompt content here"
  }'
```

**Response:**
```json
{
  "job_id": "123e4567-e89b-12d3-a456-426614174000",
  "is_new_job": false,
  "message": "Job updated successfully."
}
```

### 2. Cập nhật schedule (tạo job mới)
```bash
curl -X PUT http://localhost:3008/api/jobs/123e4567-e89b-12d3-a456-426614174000 \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "schedule": "0 9 * * *"
  }'
```

**Response:**
```json
{
  "job_id": "456e7890-e89b-12d3-a456-426614174001",
  "is_new_job": true,
  "message": "Job updated successfully. New job created due to time/target changes."
}
```

### 3. Cập nhật cả prompt và schedule
```bash
curl -X PUT http://localhost:3008/api/jobs/123e4567-e89b-12d3-a456-426614174000 \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "payload": "Updated prompt content",
    "schedule": "0 10 * * *"
  }'
```

**Response:**
```json
{
  "job_id": "789e0123-e89b-12d3-a456-426614174002",
  "is_new_job": true,
  "message": "Job updated successfully. New job created due to time/target changes."
}
```

## Logic hoạt động

### Khi nào tạo job mới:
- Thay đổi `type` (scheduled ↔ interval)
- Thay đổi `schedule` 
- Thay đổi `interval`

### Khi nào cập nhật job hiện tại:
- Chỉ thay đổi `name`
- Chỉ thay đổi `payload`

### Khi tạo job mới:
1. Tạo job mới với ID mới
2. Copy tất cả thông tin từ job cũ
3. Áp dụng các thay đổi từ request
4. Xóa job cũ (soft delete)
5. Schedule job mới trong River
6. Trả về ID của job mới

### Khi cập nhật job hiện tại:
1. Cập nhật trực tiếp các field trong database
2. Giữ nguyên ID và các thông tin khác
3. Trả về ID của job hiện tại

## Error Responses

### 400 Bad Request
```json
{
  "error": "At least one field must be provided for update"
}
```

### 401 Unauthorized
```json
{
  "error": "Unauthorized"
}
```

### 500 Internal Server Error
```json
{
  "error": "Job not found"
}
```

## Lưu ý
- Job cũ sẽ được soft delete (is_deleted = true) khi tạo job mới
- Job mới sẽ có ID mới và được schedule lại trong River
- Tất cả tasks của job cũ vẫn được giữ nguyên
- API này yêu cầu authentication thông qua JWT token