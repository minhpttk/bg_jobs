# Performance Optimization Guide

## Vấn đề N+1 Query và Giải pháp

### 🔍 **Vấn đề đã phát hiện**

Với dataset lớn (1M+ jobs, >1M tasks), các query patterns hiện tại gây ra vấn đề N+1:

1. **GetJob method**: 2 queries riêng biệt cho count và data
2. **RecoverRunningTasks**: N queries cho task updates + N queries cho job updates
3. **Indexes không tối ưu**: Thiếu covering indexes và composite indexes

### ✅ **Giải pháp đã triển khai**

#### 1. **Tối ưu Indexes (Migration 003)**

```sql
-- Covering index cho user/workspace queries
CREATE INDEX idx_jobs_user_workspace_status ON jobs (
    user_id, workspace_id, status, is_deleted
) WHERE is_deleted = false;

-- Covering index cho status/type với pagination
CREATE INDEX idx_jobs_status_type_pagination ON jobs (
    status, type, created_at DESC, is_deleted
) WHERE is_deleted = false;

-- Index cho recovery queries
CREATE INDEX idx_tasks_status_recovery ON tasks (
    status, is_deleted, created_at
) WHERE status = 'running' AND is_deleted = false;

-- Covering index cho task queries với pagination
CREATE INDEX idx_tasks_job_id_status_pagination ON tasks (
    job_id, status, created_at DESC, is_deleted
) WHERE is_deleted = false;
```

#### 2. **Batch Operations thay vì N+1**

**Trước (N+1 queries):**
```go
// N queries cho task updates
for _, task := range runningTasks {
    s.db.GORM.Model(&models.Tasks{}).
        Where("id = ?", task.ID).
        Updates(...)
}

// N queries cho job updates  
for jobID := range jobIDs {
    s.db.GORM.Model(&models.Jobs{}).
        Where("id = ?", jobID).
        Updates(...)
}
```

**Sau (2 queries):**
```go
// 1 query cho batch task updates
s.db.GORM.Model(&models.Tasks{}).
    Where("id IN ?", taskIDs).
    Updates(...)

// 1 query cho batch job updates
s.db.GORM.Model(&models.Jobs{}).
    Where("id IN ?", jobIDList).
    Updates(...)
```

#### 3. **Single Query với Window Function**

**Trước (2 queries):**
```go
// Query 1: Count
s.db.GORM.Model(&models.Tasks{}).Where(...).Count(&count)

// Query 2: Data
s.db.GORM.Where(...).Offset(...).Limit(...).Find(&tasks)
```

**Sau (1 query):**
```sql
WITH task_data AS (
    SELECT *,
           COUNT(*) OVER() as total_count
    FROM tasks 
    WHERE job_id = ? AND is_deleted = false
    ORDER BY created_at DESC
    LIMIT ? OFFSET ?
)
SELECT * FROM task_data
```

### 📊 **Performance Improvements**

| Scenario | Trước | Sau | Improvement |
|----------|-------|-----|-------------|
| 1M tasks recovery | ~2M queries | 2 queries | **99.9%** |
| GetJob với pagination | 2 queries | 1 query | **50%** |
| Index scan efficiency | O(n) | O(log n) | **Exponential** |

### 🚀 **Best Practices**

#### 1. **Sử dụng Batch Operations**
```go
// ✅ Tốt: Batch update
func (s *TasksService) BulkUpdateTaskStatuses(taskIDs []uuid.UUID, status models.TaskStatus) error {
    return s.db.GORM.Model(&models.Tasks{}).
        Where("id IN ?", taskIDs).
        Updates(map[string]interface{}{
            "status": status,
            "updated_at": time.Now(),
        }).Error
}

// ❌ Xấu: Individual updates
for _, taskID := range taskIDs {
    s.db.GORM.Model(&models.Tasks{}).Where("id = ?", taskID).Updates(...)
}
```

#### 2. **Covering Indexes**
```sql
-- ✅ Tốt: Covering index với tất cả columns cần thiết
CREATE INDEX idx_tasks_job_id_status_pagination ON tasks (
    job_id, status, created_at DESC, is_deleted
) WHERE is_deleted = false;

-- ❌ Xấu: Index không covering
CREATE INDEX idx_tasks_job_id ON tasks (job_id);
```

#### 3. **Window Functions cho Pagination**
```sql
-- ✅ Tốt: Single query với count
WITH task_data AS (
    SELECT *, COUNT(*) OVER() as total_count
    FROM tasks 
    WHERE job_id = ? AND is_deleted = false
    ORDER BY created_at DESC
    LIMIT ? OFFSET ?
)
SELECT * FROM task_data

-- ❌ Xấu: Separate count and data queries
SELECT COUNT(*) FROM tasks WHERE job_id = ?;
SELECT * FROM tasks WHERE job_id = ? LIMIT ? OFFSET ?;
```

### 🔧 **Monitoring và Maintenance**

#### 1. **Query Performance Monitoring**
```sql
-- Kiểm tra index usage
SELECT schemaname, tablename, indexname, idx_scan, idx_tup_read, idx_tup_fetch
FROM pg_stat_user_indexes
WHERE tablename IN ('jobs', 'tasks')
ORDER BY idx_scan DESC;

-- Kiểm tra slow queries
SELECT query, calls, total_time, mean_time
FROM pg_stat_statements
WHERE query LIKE '%tasks%' OR query LIKE '%jobs%'
ORDER BY mean_time DESC
LIMIT 10;
```

#### 2. **Index Maintenance**
```sql
-- Rebuild indexes định kỳ
REINDEX INDEX CONCURRENTLY idx_tasks_job_id_status_pagination;
REINDEX INDEX CONCURRENTLY idx_jobs_user_workspace_status;

-- Analyze tables
ANALYZE jobs;
ANALYZE tasks;
```

### 📈 **Expected Performance với 1M+ Records**

| Operation | Records | Trước | Sau | Time Saved |
|-----------|---------|-------|-----|------------|
| Task Recovery | 10K tasks | ~20s | ~0.2s | **99%** |
| GetJob pagination | 100K tasks | ~5s | ~0.1s | **98%** |
| Bulk status update | 50K tasks | ~50s | ~0.5s | **99%** |

### 🎯 **Next Steps**

1. **Deploy migration 003** để apply optimized indexes
2. **Monitor query performance** sau khi deploy
3. **Implement caching** cho frequently accessed data
4. **Consider read replicas** cho read-heavy operations
5. **Implement connection pooling** nếu chưa có