-- Migration 002: Add role column to users and user_id to offline_tasks and file_cache

ALTER TABLE users ADD COLUMN role TEXT NOT NULL DEFAULT 'user';
UPDATE users SET role = 'admin' WHERE id = 1 OR username = 'admin';

ALTER TABLE offline_tasks ADD COLUMN user_id INTEGER NOT NULL DEFAULT 1;
CREATE INDEX IF NOT EXISTS idx_offline_tasks_user_id ON offline_tasks(user_id);

ALTER TABLE file_cache ADD COLUMN user_id INTEGER NOT NULL DEFAULT 1;
CREATE INDEX IF NOT EXISTS idx_file_cache_user_id ON file_cache(user_id);
