export interface Account {
  id: number
  name: string
  username: string
  proxy_url: string
  priority: number
  is_enabled: boolean
  status: string
  quota_exhausted_at?: string
  cooldown_until?: string
  daily_task_count: number
  last_task_date?: string
  total_files: number
  used_space: number
  total_space: number
  last_login_at?: string
  last_success_at?: string
  last_error?: string
  created_at: string
  updated_at: string
  has_password: boolean
  has_refresh_token: boolean
}

export interface VirtualFile {
  virtual_id: string
  account_id: number
  account_name: string
  pikpak_file_id: string
  parent_id: string
  name: string
  size: number
  mime_type: string
  kind: string
  is_folder: boolean
  is_video: boolean
  thumbnail_link: string
  created_time: string
  modified_time: string
}

export interface OfflineTask {
  id: string
  source_url: string
  file_name: string
  account_id: number
  account_name: string
  pikpak_task_id: string
  pikpak_file_id: string
  status: string
  progress: number
  error_message?: string
  created_at: string
  updated_at: string
  completed_at?: string
}

export interface APIKey {
  id: number
  name: string
  key_prefix: string
  permissions: string
  is_enabled: boolean
  last_used_at?: string
  created_at: string
  full_key?: string
}

export interface AuditLog {
  id: number
  action: string
  target: string
  details: string
  status: string
  operator: string
  created_at: string
}

export interface DashboardStats {
  total_accounts: number
  healthy_accounts: number
  quota_exhausted_count: number
  auth_failed_count: number
  proxy_failed_count: number
  cooldown_count: number
  disabled_count: number
  total_tasks: number
  running_tasks: number
  completed_tasks: number
  failed_tasks: number
  total_files: number
  used_space: number
  total_space: number
}

export interface PlaybackInfo {
  virtual_id: string
  name: string
  size: number
  mime_type: string
  is_video: boolean
  direct_url: string
  proxy_url: string
  account_name: string
}

async function request<T>(url: string, options: RequestInit = {}): Promise<T> {
  const res = await fetch(url, {
    ...options,
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...options.headers,
    },
  })

  if (!res.ok) {
    let errMsg = `Request failed: ${res.status}`
    try {
      const errObj = await res.json()
      if (errObj.error) errMsg = errObj.error
    } catch {
      // ignore
    }
    throw new Error(errMsg)
  }

  return res.json()
}

export const api = {
  // Auth
  login: (username: string, password: string) =>
    request<{ success: boolean; token: string; username: string }>("/api/auth/login", {
      method: "POST",
      body: JSON.stringify({ username, password }),
    }),
  logout: () => request<{ success: boolean }>("/api/auth/logout", { method: "POST" }),
  getMe: () => request<{ user_id: number; username: string }>("/api/auth/me"),
  changePassword: (old_password: string, new_password: string) =>
    request<{ success: boolean }>("/api/auth/change-password", {
      method: "POST",
      body: JSON.stringify({ old_password, new_password }),
    }),

  // Dashboard
  getDashboardStats: () => request<DashboardStats>("/api/dashboard/stats"),

  // Accounts
  listAccounts: () => request<Account[]>("/api/accounts"),
  createAccount: (data: any) =>
    request<Account>("/api/accounts", { method: "POST", body: JSON.stringify(data) }),
  updateAccount: (id: number, data: any) =>
    request<Account>(`/api/accounts/${id}`, { method: "PUT", body: JSON.stringify(data) }),
  deleteAccount: (id: number) =>
    request<{ success: boolean }>(`/api/accounts/${id}`, { method: "DELETE" }),
  testAccount: (id: number) =>
    request<{ success: boolean; status: string; used_space: number; total_space: number; latency_ms: number; error?: string }>(
      `/api/accounts/${id}/test`,
      { method: "POST" }
    ),
  resetQuota: (id: number) =>
    request<{ success: boolean }>(`/api/accounts/${id}/reset-quota`, { method: "POST" }),
  testProxy: (proxy_url: string) =>
    request<{ success: boolean; latency_ms: number; egress_ip: string; error?: string }>(
      "/api/accounts/test-proxy",
      { method: "POST", body: JSON.stringify({ proxy_url }) }
    ),

  // Files
  listFiles: (parent_id = "", sort_by = "name", sort_order = "asc") =>
    request<VirtualFile[]>(
      `/api/files?parent_id=${encodeURIComponent(parent_id)}&sort_by=${sort_by}&sort_order=${sort_order}`
    ),
  searchFiles: (keyword: string, account_id?: number) =>
    request<VirtualFile[]>(
      `/api/files/search?keyword=${encodeURIComponent(keyword)}${account_id ? `&account_id=${account_id}` : ""}`
    ),
  batchDelete: (virtual_ids: string[], permanent = false) =>
    request<{ total: number; success: number; failed: number; items: { virtual_id: string; success: boolean; error?: string }[] }>(
      "/api/files/batch-delete",
      { method: "POST", body: JSON.stringify({ virtual_ids, permanent }) }
    ),

  // Media
  getPlaybackInfo: (virtual_id: string) => request<PlaybackInfo>(`/api/media/info/${virtual_id}`),

  // Offline Tasks
  listTasks: (status = "", limit = 50, offset = 0) =>
    request<{ tasks: OfflineTask[]; total: number; limit: number; offset: number }>(
      `/api/offline/tasks?status=${status}&limit=${limit}&offset=${offset}`
    ),
  getTask: (id: string) => request<OfflineTask>(`/api/offline/tasks/${id}`),
  submitOffline: (urls: string[] | string, name = "") => {
    const payload = typeof urls === "string" ? { url: urls, name } : { urls, name }
    return request<{ success: boolean; results?: any[]; task_id?: string; status?: string }>("/api/offline/tasks", {
      method: "POST",
      body: JSON.stringify(payload),
    })
  },
  deleteTask: (id: string) =>
    request<{ success: boolean }>(`/api/offline/tasks/${id}`, { method: "DELETE" }),
  cancelTask: (id: string) =>
    request<{ success: boolean }>(`/api/offline/tasks/${id}/cancel`, { method: "POST" }),
  retryTask: (id: string) =>
    request<any>(`/api/offline/tasks/${id}/retry`, { method: "POST" }),

  // API Keys
  listApiKeys: () => request<APIKey[]>("/api/apikeys"),
  createApiKey: (name: string, permissions: string) =>
    request<APIKey>("/api/apikeys", {
      method: "POST",
      body: JSON.stringify({ name, permissions }),
    }),
  deleteApiKey: (id: number) =>
    request<{ success: boolean }>(`/api/apikeys/${id}`, { method: "DELETE" }),
  toggleApiKey: (id: number, enabled: boolean) =>
    request<{ success: boolean }>(`/api/apikeys/${id}/toggle`, {
      method: "POST",
      body: JSON.stringify({ enabled }),
    }),

  // Audit Logs
  listAuditLogs: (limit = 50, offset = 0) =>
    request<{ logs: AuditLog[]; total: number }>(`/api/audit/logs?limit=${limit}&offset=${offset}`),
}
