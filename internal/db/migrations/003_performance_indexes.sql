-- Migration 003: Performance indexes for high-frequency queries
CREATE INDEX IF NOT EXISTS idx_file_cache_user_parent ON file_cache(user_id, parent_id);
CREATE INDEX IF NOT EXISTS idx_offline_tasks_user_created ON offline_tasks(user_id, created_at DESC);
