# Task Recovery Feature

## Tổng quan

Tính năng Task Recovery được thêm vào để giải quyết vấn đề khi server restart giữa chừng trong quá trình thực hiện job interval. Khi server restart, các task đang ở trạng thái `running` sẽ được khôi phục về trạng thái `created` để có thể thực hiện lại.

## Vấn đề được giải quyết

Trước đây, khi server restart trong lúc một job interval đang thực hiện (ví dụ: đang thực hiện task thứ 2 trong 5 task), các task còn lại sẽ bị miss và job sẽ bắt đầu lại từ đầu khi server restart xong.

Với tính năng Task Recovery:
- Các task đang `running` sẽ được khôi phục về trạng thái `created`
- Job sẽ tiếp tục thực hiện các task còn lại thay vì bắt đầu lại từ đầu
- Không ảnh hưởng đến logic job recovery hiện tại

## Cách hoạt động

### 1. Theo dõi task hiện tại
- Thêm field `current_task_id` vào bảng `jobs` để theo dõi task đang thực hiện
- Khi job bắt đầu thực hiện task, `current_task_id` được cập nhật
- Khi task hoàn thành hoặc thất bại, `current_task_id` được clear

### 2. Task Recovery Process
Khi server khởi động:
1. **Task Recovery**: Tìm tất cả task có status `running` và reset về `created`
2. **Clear Current Task ID**: Clear `current_task_id` của các job có task đang running
3. **Job Recovery**: Tiếp tục với logic job recovery hiện tại

### 3. Thứ tự thực hiện
```
Server Start → Task Recovery → Job Recovery → Normal Operation
```

## Files được thay đổi

### Models
- `models/jobs.go`: Thêm field `CurrentTaskID`

### Services
- `services/task_service.go`: 
  - `RecoverRunningTasks()`: Khôi phục task đang running
  - `GetIncompleteTasksByJobID()`: Lấy task chưa hoàn thành
- `services/job_service.go`:
  - `UpdateCurrentTaskID()`: Cập nhật task hiện tại
  - `HasRunningTasks()`: Kiểm tra job có task đang chạy

### Workers
- `services/job_workers.go`: Cập nhật để theo dõi `current_task_id`

### Migration
- `migrations/002_add_current_task_id_to_jobs.sql`: Thêm column `current_task_id`
- `migrations/migrate.go`: Thêm SQL migration runner

### Main Application
- `cmd/worker/main.go`: Thêm task recovery vào startup process

## Database Changes

### New Column
```sql
ALTER TABLE jobs ADD COLUMN current_task_id UUID;
CREATE INDEX idx_jobs_current_task_id ON jobs (current_task_id) WHERE current_task_id IS NOT NULL;
```

## Testing

Chạy test script để kiểm tra tính năng:
```bash
./scripts/test_task_recovery.sh
```

## Logs

Khi task recovery hoạt động, bạn sẽ thấy logs như:
```
Starting task recovery process...
Found 3 running tasks to recover
Recovered task abc-123 (job: def-456) from running to created status
Cleared current_task_id for job def-456
Task recovery completed. Recovered 3 tasks from 1 jobs
Task recovery completed successfully
```

## Lưu ý

1. **Không ảnh hưởng logic cũ**: Task recovery hoạt động độc lập với job recovery hiện tại
2. **Performance**: Chỉ chạy một lần khi server start, không ảnh hưởng performance runtime
3. **Data Integrity**: Clear partial results khi recovery để đảm bảo task chạy lại từ đầu
4. **Backward Compatibility**: Tương thích với dữ liệu cũ, field `current_task_id` có thể NULL