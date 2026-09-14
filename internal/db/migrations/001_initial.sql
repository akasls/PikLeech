-- Schema migrations table is handled by the migration runner

CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS pikpak_accounts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    username TEXT,
    password_enc TEXT,
    refresh_token_enc TEXT,
    access_token_enc TEXT,
    user_id TEXT,
    device_id TEXT,
    captcha_token TEXT,
    proxy_url TEXT,
    priority INTEGER DEFAULT 10,
    is_enabled BOOLEAN DEFAULT 1,
    status TEXT NOT NULL DEFAULT 'HEALTHY',
    quota_exhausted_at DATETIME,
    cooldown_until DATETIME,
    daily_task_count INTEGER DEFAULT 0,
    last_task_date TEXT,
    total_files INTEGER DEFAULT 0,
    used_space INTEGER DEFAULT 0,
    total_space INTEGER DEFAULT 0,
    last_login_at DATETIME,
    last_success_at DATETIME,
    last_error TEXT,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS offline_tasks (
    id TEXT PRIMARY KEY,
    source_url TEXT NOT NULL,
    file_name TEXT,
    account_id INTEGER NOT NULL,
    pikpak_task_id TEXT,
    pikpak_file_id TEXT,
    status TEXT NOT NULL DEFAULT 'PENDING',
    progress INTEGER DEFAULT 0,
    error_message TEXT,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    completed_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_offline_tasks_status ON offline_tasks(status);
CREATE INDEX IF NOT EXISTS idx_offline_tasks_account_id ON offline_tasks(account_id);

CREATE TABLE IF NOT EXISTS api_keys (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    key_hash TEXT NOT NULL UNIQUE,
    key_prefix TEXT NOT NULL,
    permissions TEXT NOT NULL,
    is_enabled BOOLEAN DEFAULT 1,
    last_used_at DATETIME,
    created_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS file_cache (
    virtual_id TEXT PRIMARY KEY,
    account_id INTEGER NOT NULL,
    pikpak_file_id TEXT NOT NULL,
    parent_id TEXT NOT NULL,
    name TEXT NOT NULL,
    size INTEGER DEFAULT 0,
    mime_type TEXT,
    kind TEXT NOT NULL,
    thumbnail_link TEXT,
    created_time DATETIME,
    modified_time DATETIME,
    updated_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_file_cache_parent_id ON file_cache(parent_id);
CREATE INDEX IF NOT EXISTS idx_file_cache_account_id ON file_cache(account_id);
CREATE INDEX IF NOT EXISTS idx_file_cache_name ON file_cache(name);

CREATE TABLE IF NOT EXISTS idempotency_keys (
    key TEXT PRIMARY KEY,
    response_json TEXT NOT NULL,
    created_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS audit_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    action TEXT NOT NULL,
    target TEXT NOT NULL,
    details TEXT,
    status TEXT NOT NULL,
    operator TEXT NOT NULL,
    created_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at DESC);

CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
