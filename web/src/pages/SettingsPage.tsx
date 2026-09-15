import React, { useState, useEffect } from "react"
import {
  LayoutDashboard,
  Users,
  Key,
  UserCheck,
  UserPlus,
  ShieldAlert,
  Loader2,
  Server,
  DownloadCloud,
  CheckCircle2,
  RefreshCw,
  HardDrive,
  RotateCcw,
  AlertCircle,
  Trash2,
  Lock,
  User,
  Check,
  LogOut,
  Sliders,
  Eraser,
  X,
  Info,
  ExternalLink,
  Github,
  Copy,
} from "lucide-react"
import { api, DashboardStats, OfflineTask, SystemSettings, UserInfo } from "../lib/api"
import { formatBytes, formatDate } from "../lib/utils"
import { AccountsPage } from "./AccountsPage"
import { ApiKeysPage } from "./ApiKeysPage"
import { AuditPage } from "./AuditPage"

export type SettingsTab = "overview" | "policy" | "accounts" | "users" | "apikeys" | "profile" | "audit" | "about"

interface SettingsPageProps {
  currentUsername: string
  currentUserRole?: string
  onUpdateUsername: (newUsername: string) => void
  onOpenNewOffline?: () => void
  onLogout?: () => void
}

export const SettingsPage: React.FC<SettingsPageProps> = ({
  currentUsername,
  currentUserRole = "admin",
  onUpdateUsername,
  onOpenNewOffline,
  onLogout,
}) => {
  const isAdmin = currentUserRole === "admin" || currentUsername === "admin"
  const [activeTab, setActiveTab] = useState<SettingsTab>(() =>
    currentUserRole === "admin" || currentUsername === "admin" ? "overview" : "profile"
  )

  useEffect(() => {
    if (!isAdmin && activeTab !== "profile" && activeTab !== "about") {
      setActiveTab("profile")
    }
  }, [isAdmin, activeTab])

  const [copiedRepo, setCopiedRepo] = useState(false)

  // Overview states
  const [stats, setStats] = useState<DashboardStats | null>(null)
  const [tasks, setTasks] = useState<OfflineTask[]>([])
  const [loadingOverview, setLoadingOverview] = useState(true)
  const [statusFilter, setStatusFilter] = useState("")

  // Policy states
  const [settings, setSettings] = useState<SystemSettings>({
    storage_balancing_enabled: true,
    storage_min_free_gb: 10,
    auto_cleanup_enabled: false,
    auto_cleanup_days: 7,
  })
  const [savingSettings, setSavingSettings] = useState(false)
  const [settingsMsg, setSettingsMsg] = useState<{ text: string; isError?: boolean } | null>(null)
  const [cleaningNow, setCleaningNow] = useState(false)
  const [cleanupFeedback, setCleanupFeedback] = useState<{ message: string; isError?: boolean } | null>(null)

  // Users management states
  const [userList, setUserList] = useState<UserInfo[]>([])
  const [loadingUsers, setLoadingUsers] = useState(false)
  const [showAddUserModal, setShowAddUserModal] = useState(false)
  const [addUsername, setAddUsername] = useState("")
  const [addPassword, setAddPassword] = useState("")
  const [addRole, setAddRole] = useState<string>("user")
  const [addingUser, setAddingUser] = useState(false)
  const [addUserError, setAddUserError] = useState("")

  // Reset password states
  const [resetUser, setResetUser] = useState<UserInfo | null>(null)
  const [resetPassword, setResetPassword] = useState("")
  const [resettingPassword, setResettingPassword] = useState(false)
  const [resetError, setResetError] = useState("")

  // Profile states
  const [newUsername, setNewUsername] = useState(currentUsername)
  const [oldPassword, setOldPassword] = useState("")
  const [newPassword, setNewPassword] = useState("")
  const [confirmPassword, setConfirmPassword] = useState("")
  const [profileSaving, setProfileSaving] = useState(false)
  const [profileMsg, setProfileMsg] = useState<{ text: string; isError?: boolean } | null>(null)

  const loadOverviewData = async () => {
    if (!isAdmin) return
    setLoadingOverview(true)
    try {
      const [sData, tData] = await Promise.all([
        api.getDashboardStats(),
        api.listTasks(statusFilter, 50, 0),
      ])
      setStats(sData)
      setTasks(tData.tasks || [])
    } catch (err) {
      console.error(err)
    } finally {
      setLoadingOverview(false)
    }
  }

  useEffect(() => {
    if (activeTab === "overview" && isAdmin) {
      loadOverviewData()
      const timer = setInterval(() => {
        api.listTasks(statusFilter, 50, 0).then((d) => setTasks(d.tasks || []))
        api.getDashboardStats().then((s) => setStats(s))
      }, 5000)
      return () => clearInterval(timer)
    }
  }, [activeTab, statusFilter, isAdmin])

  const loadSettings = async () => {
    if (!isAdmin) return
    try {
      const res = await api.getSettings()
      if (res) setSettings(res)
    } catch (err: any) {
      console.error("Failed to load settings:", err)
    }
  }

  useEffect(() => {
    if (activeTab === "policy" && isAdmin) {
      loadSettings()
    }
  }, [activeTab, isAdmin])

  const loadUsers = async () => {
    if (!isAdmin) return
    setLoadingUsers(true)
    try {
      const res = await api.listUsers()
      if (res && res.users) {
        setUserList(res.users)
      }
    } catch (err: any) {
      console.error("Failed to load users:", err)
    } finally {
      setLoadingUsers(false)
    }
  }

  useEffect(() => {
    if (activeTab === "users" && isAdmin) {
      loadUsers()
    }
  }, [activeTab, isAdmin])

  const handleSaveSettings = async (e: React.FormEvent) => {
    e.preventDefault()
    setSavingSettings(true)
    setSettingsMsg(null)
    try {
      await api.updateSettings(settings)
      setSettingsMsg({ text: "存储自动均衡与清理策略已成功保存生效！" })
    } catch (err: any) {
      setSettingsMsg({ text: err.message || "保存失败", isError: true })
    } finally {
      setSavingSettings(false)
    }
  }

  const handleTriggerCleanupNow = async () => {
    setCleaningNow(true)
    setCleanupFeedback(null)
    try {
      const res = await api.triggerCleanup(settings.auto_cleanup_days)
      setCleanupFeedback({ message: res.message || `已成功清理 ${res.cleaned_tasks_count} 个过期任务！` })
      loadOverviewData()
    } catch (err: any) {
      setCleanupFeedback({ message: err.message || "清理失败", isError: true })
    } finally {
      setCleaningNow(false)
    }
  }

  const handleCreateUser = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!addUsername.trim() || !addPassword) return
    setAddingUser(true)
    setAddUserError("")
    try {
      await api.createUser({
        username: addUsername.trim(),
        password: addPassword,
        role: addRole,
      })
      setShowAddUserModal(false)
      setAddUsername("")
      setAddPassword("")
      setAddRole("user")
      loadUsers()
    } catch (err: any) {
      setAddUserError(err.message || "创建用户失败")
    } finally {
      setAddingUser(false)
    }
  }

  const handleDeleteUser = async (u: UserInfo) => {
    if (u.username === currentUsername) {
      alert("无法删除当前登录的用户账号")
      return
    }
    if (!confirm(`确定要彻底删除用户 "${u.username}" 吗？该用户的所有独立文件和任务将不再可用。`)) return
    try {
      await api.deleteUser(u.id)
      loadUsers()
    } catch (err: any) {
      alert("删除失败: " + err.message)
    }
  }

  const handleResetPassword = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!resetUser || !resetPassword) return
    setResettingPassword(true)
    setResetError("")
    try {
      await api.resetUserPassword(resetUser.id, resetPassword)
      const targetName = resetUser.username
      setResetUser(null)
      setResetPassword("")
      alert(`用户 "${targetName}" 的密码已重置成功！`)
    } catch (err: any) {
      setResetError(err.message || "重置密码失败")
    } finally {
      setResettingPassword(false)
    }
  }

  useEffect(() => {
    setNewUsername(currentUsername)
  }, [currentUsername])

  const handleCancelTask = async (id: string) => {
    try {
      await api.cancelTask(id)
      loadOverviewData()
    } catch (err: any) {
      alert("取消任务失败: " + err.message)
    }
  }

  const handleRetryTask = async (id: string) => {
    try {
      await api.retryTask(id)
      loadOverviewData()
    } catch (err: any) {
      alert("重试任务失败: " + err.message)
    }
  }

  const handleDeleteTask = async (id: string) => {
    try {
      await api.deleteTask(id)
      setTasks((prev) => prev.filter((t) => t.id !== id))
    } catch (err: any) {
      alert("删除记录失败: " + err.message)
    }
  }

  const handleSaveProfile = async (e: React.FormEvent) => {
    e.preventDefault()
    setProfileMsg(null)

    if (newPassword && newPassword !== confirmPassword) {
      setProfileMsg({ text: "两次输入的新密码不一致", isError: true })
      return
    }

    if (newPassword && newPassword.length < 6) {
      setProfileMsg({ text: "新密码长度至少需要 6 个字符", isError: true })
      return
    }

    setProfileSaving(true)
    try {
      const res = await api.updateProfile({
        username: newUsername,
        old_password: oldPassword,
        new_password: newPassword,
      })
      setProfileMsg({ text: "个人信息修改成功！" })
      if (res.username) {
        onUpdateUsername(res.username)
      }
      setOldPassword("")
      setNewPassword("")
      setConfirmPassword("")
    } catch (err: any) {
      setProfileMsg({ text: err.message || "修改失败", isError: true })
    } finally {
      setProfileSaving(false)
    }
  }

  const storagePercent =
    stats && stats.total_space > 0
      ? Math.min(100, Math.round((stats.used_space / stats.total_space) * 100))
      : 0

  const getStatusBadge = (status: string) => {
    switch (status) {
      case "COMPLETE":
        return (
          <span className="inline-flex items-center gap-1 rounded-full bg-emerald-500/15 px-2.5 py-0.5 text-xs font-medium text-emerald-600 dark:text-emerald-400">
            <CheckCircle2 className="h-3.5 w-3.5" />
            已完成
          </span>
        )
      case "RUNNING":
      case "PENDING":
        return (
          <span className="inline-flex items-center gap-1 rounded-full bg-blue-500/15 px-2.5 py-0.5 text-xs font-medium text-blue-600 dark:text-blue-400">
            <Loader2 className="h-3.5 w-3.5 animate-spin" />
            下载中
          </span>
        )
      case "ERROR":
        return (
          <span className="inline-flex items-center gap-1 rounded-full bg-destructive/15 px-2.5 py-0.5 text-xs font-medium text-destructive">
            <AlertCircle className="h-3.5 w-3.5" />
            失败
          </span>
        )
      case "CANCELLED":
        return (
          <span className="inline-flex items-center gap-1 rounded-full bg-secondary px-2.5 py-0.5 text-xs font-medium text-muted-foreground">
            已取消
          </span>
        )
      default:
        return (
          <span className="inline-flex items-center gap-1 rounded-full bg-secondary px-2.5 py-0.5 text-xs font-medium text-muted-foreground">
            {status}
          </span>
        )
    }
  }

  return (
    <div className="space-y-4 no-scrollbar">
      {/* Top Menu Bar with Tabs and right-aligned Logout */}
      <div className="flex items-center justify-between gap-2 overflow-x-auto no-scrollbar rounded-2xl border bg-card p-1 sm:p-1.5 shadow-sm">
        <div className="flex items-center gap-1 sm:gap-1.5 overflow-x-auto no-scrollbar">
          {isAdmin ? (
            <>
              <button
                onClick={() => setActiveTab("overview")}
                className={`flex items-center gap-1.5 sm:gap-2 rounded-xl px-3 sm:px-4 py-1.5 sm:py-2 text-xs sm:text-sm font-medium transition-all whitespace-nowrap ${
                  activeTab === "overview"
                    ? "bg-primary text-primary-foreground shadow-sm font-semibold"
                    : "text-muted-foreground hover:text-foreground hover:bg-secondary/60"
                }`}
              >
                <LayoutDashboard className="h-4 w-4 shrink-0" />
                <span>系统预览</span>
              </button>

              <button
                onClick={() => setActiveTab("policy")}
                className={`flex items-center gap-1.5 sm:gap-2 rounded-xl px-3 sm:px-4 py-1.5 sm:py-2 text-xs sm:text-sm font-medium transition-all whitespace-nowrap ${
                  activeTab === "policy"
                    ? "bg-primary text-primary-foreground shadow-sm font-semibold"
                    : "text-muted-foreground hover:text-foreground hover:bg-secondary/60"
                }`}
              >
                <Sliders className="h-4 w-4 shrink-0" />
                <span>存储与自动策略</span>
              </button>

              <button
                onClick={() => setActiveTab("accounts")}
                className={`flex items-center gap-1.5 sm:gap-2 rounded-xl px-3 sm:px-4 py-1.5 sm:py-2 text-xs sm:text-sm font-medium transition-all whitespace-nowrap ${
                  activeTab === "accounts"
                    ? "bg-primary text-primary-foreground shadow-sm font-semibold"
                    : "text-muted-foreground hover:text-foreground hover:bg-secondary/60"
                }`}
              >
                <Users className="h-4 w-4 shrink-0" />
                <span>账号管理</span>
              </button>

              <button
                onClick={() => setActiveTab("users")}
                className={`flex items-center gap-1.5 sm:gap-2 rounded-xl px-3 sm:px-4 py-1.5 sm:py-2 text-xs sm:text-sm font-medium transition-all whitespace-nowrap ${
                  activeTab === "users"
                    ? "bg-primary text-primary-foreground shadow-sm font-semibold"
                    : "text-muted-foreground hover:text-foreground hover:bg-secondary/60"
                }`}
              >
                <UserPlus className="h-4 w-4 shrink-0" />
                <span>用户管理</span>
              </button>

              <button
                onClick={() => setActiveTab("apikeys")}
                className={`flex items-center gap-1.5 sm:gap-2 rounded-xl px-3 sm:px-4 py-1.5 sm:py-2 text-xs sm:text-sm font-medium transition-all whitespace-nowrap ${
                  activeTab === "apikeys"
                    ? "bg-primary text-primary-foreground shadow-sm font-semibold"
                    : "text-muted-foreground hover:text-foreground hover:bg-secondary/60"
                }`}
              >
                <Key className="h-4 w-4 shrink-0" />
                <span>API管理</span>
              </button>

              <button
                onClick={() => setActiveTab("profile")}
                className={`flex items-center gap-1.5 sm:gap-2 rounded-xl px-3 sm:px-4 py-1.5 sm:py-2 text-xs sm:text-sm font-medium transition-all whitespace-nowrap ${
                  activeTab === "profile"
                    ? "bg-primary text-primary-foreground shadow-sm font-semibold"
                    : "text-muted-foreground hover:text-foreground hover:bg-secondary/60"
                }`}
              >
                <UserCheck className="h-4 w-4 shrink-0" />
                <span>管理员信息</span>
              </button>

              <button
                onClick={() => setActiveTab("audit")}
                className={`flex items-center gap-1.5 sm:gap-2 rounded-xl px-3 sm:px-4 py-1.5 sm:py-2 text-xs sm:text-sm font-medium transition-all whitespace-nowrap ${
                  activeTab === "audit"
                    ? "bg-primary text-primary-foreground shadow-sm font-semibold"
                    : "text-muted-foreground hover:text-foreground hover:bg-secondary/60"
                }`}
              >
                <ShieldAlert className="h-4 w-4 shrink-0" />
                <span>审计日志</span>
              </button>

              <button
                onClick={() => setActiveTab("about")}
                className={`flex items-center gap-1.5 sm:gap-2 rounded-xl px-3 sm:px-4 py-1.5 sm:py-2 text-xs sm:text-sm font-medium transition-all whitespace-nowrap ${
                  activeTab === "about"
                    ? "bg-primary text-primary-foreground shadow-sm font-semibold"
                    : "text-muted-foreground hover:text-foreground hover:bg-secondary/60"
                }`}
              >
                <Info className="h-4 w-4 shrink-0" />
                <span>关于项目</span>
              </button>
            </>
          ) : (
            <>
              <button
                onClick={() => setActiveTab("profile")}
                className={`flex items-center gap-1.5 sm:gap-2 rounded-xl px-3 sm:px-4 py-1.5 sm:py-2 text-xs sm:text-sm font-medium transition-all whitespace-nowrap ${
                  activeTab === "profile"
                    ? "bg-primary text-primary-foreground shadow-sm font-semibold"
                    : "text-muted-foreground hover:text-foreground hover:bg-secondary/60"
                }`}
              >
                <User className="h-4 w-4 shrink-0" />
                <span>个人设置</span>
              </button>

              <button
                onClick={() => setActiveTab("about")}
                className={`flex items-center gap-1.5 sm:gap-2 rounded-xl px-3 sm:px-4 py-1.5 sm:py-2 text-xs sm:text-sm font-medium transition-all whitespace-nowrap ${
                  activeTab === "about"
                    ? "bg-primary text-primary-foreground shadow-sm font-semibold"
                    : "text-muted-foreground hover:text-foreground hover:bg-secondary/60"
                }`}
              >
                <Info className="h-4 w-4 shrink-0" />
                <span>关于项目</span>
              </button>
            </>
          )}
        </div>

        {onLogout && (
          <button
            onClick={onLogout}
            className="flex items-center gap-1.5 shrink-0 rounded-xl border border-destructive/25 bg-destructive/5 hover:bg-destructive/15 text-destructive px-3 py-1.5 text-xs font-medium transition-colors ml-auto mr-1"
            title="退出当前登录"
          >
            <LogOut className="h-3.5 w-3.5" />
            <span className="hidden sm:inline">退出登录</span>
          </button>
        )}
      </div>

      {/* Tab Content Area */}
      {isAdmin && activeTab === "overview" && (
        <div className="space-y-6">
          {/* Storage & Account Summary Grid */}
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {/* Account Pool */}
            <div className="rounded-2xl border bg-card p-5 shadow-sm">
              <div className="flex items-center justify-between">
                <span className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                  账号池状态
                </span>
                <Server className="h-4 w-4 text-primary" />
              </div>
              <div className="mt-3 flex items-baseline gap-2">
                <span className="text-2xl font-black">{stats?.healthy_accounts ?? 0}</span>
                <span className="text-xs text-muted-foreground">/ {stats?.total_accounts ?? 0} 正常账号</span>
              </div>
            </div>

            {/* Storage Usage */}
            <div className="rounded-2xl border bg-card p-5 shadow-sm">
              <div className="flex items-center justify-between">
                <span className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                  聚合云盘总容量
                </span>
                <HardDrive className="h-4 w-4 text-primary" />
              </div>
              <div className="mt-3 flex items-baseline gap-2">
                <span className="text-2xl font-black">
                  {formatBytes(stats?.used_space ?? 0)}
                </span>
                <span className="text-xs text-muted-foreground">
                  / {formatBytes(stats?.total_space ?? 0)}
                </span>
              </div>
              <div className="mt-3 h-1.5 w-full rounded-full bg-secondary overflow-hidden">
                <div
                  className="h-full rounded-full bg-primary transition-all duration-500"
                  style={{ width: `${storagePercent}%` }}
                />
              </div>
            </div>

            {/* Today Offline Tasks */}
            <div className="rounded-2xl border bg-card p-5 shadow-sm sm:col-span-2 lg:col-span-1">
              <div className="flex items-center justify-between">
                <span className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                  今日离线调度
                </span>
                <DownloadCloud className="h-4 w-4 text-primary" />
              </div>
              <div className="mt-3 flex items-baseline gap-2">
                <span className="text-2xl font-black">{stats?.total_tasks ?? 0}</span>
                <span className="text-xs text-muted-foreground">次调度</span>
              </div>
            </div>
          </div>

          {/* Offline Task List Section */}
          <div className="rounded-2xl border bg-card shadow-sm overflow-hidden">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 p-4 sm:p-5 border-b">
              <div className="flex items-center gap-2">
                <DownloadCloud className="h-5 w-5 text-primary" />
                <h2 className="font-bold text-sm sm:text-base">全网盘离线任务实时监控</h2>
                <span className="text-xs text-muted-foreground">({tasks.length} 任务)</span>
              </div>

              <div className="flex items-center gap-2 flex-wrap sm:flex-nowrap">
                <select
                  value={statusFilter}
                  onChange={(e) => setStatusFilter(e.target.value)}
                  className="rounded-xl border bg-background px-3 py-1.5 text-xs focus:outline-none focus:ring-1 focus:ring-primary"
                >
                  <option value="">全部状态</option>
                  <option value="RUNNING">下载中 (RUNNING)</option>
                  <option value="COMPLETE">已完成 (COMPLETE)</option>
                  <option value="ERROR">失败 (ERROR)</option>
                  <option value="CANCELLED">已取消 (CANCELLED)</option>
                </select>

                <button
                  onClick={loadOverviewData}
                  disabled={loadingOverview}
                  className="flex items-center gap-1.5 rounded-xl border px-3 py-1.5 text-xs font-medium hover:bg-secondary transition-colors"
                >
                  <RefreshCw className={`h-3.5 w-3.5 ${loadingOverview ? "animate-spin" : ""}`} />
                  刷新
                </button>

                {onOpenNewOffline && (
                  <button
                    onClick={onOpenNewOffline}
                    className="flex items-center gap-1.5 rounded-xl bg-primary px-3.5 py-1.5 text-xs font-medium text-primary-foreground shadow hover:bg-primary/90 transition-colors"
                  >
                    <DownloadCloud className="h-3.5 w-3.5" />
                    新建离线
                  </button>
                )}
              </div>
            </div>

            {loadingOverview && tasks.length === 0 ? (
              <div className="flex items-center justify-center p-12 text-muted-foreground gap-2">
                <Loader2 className="h-5 w-5 animate-spin text-primary" />
                <span className="text-xs font-medium">正在拉取全网盘离线进度...</span>
              </div>
            ) : tasks.length === 0 ? (
              <div className="flex flex-col items-center justify-center p-12 text-center text-muted-foreground space-y-2">
                <DownloadCloud className="h-8 w-8 stroke-1 text-muted-foreground/40" />
                <p className="text-xs">暂无符合条件的离线下载任务</p>
              </div>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-left text-xs border-collapse">
                  <thead>
                    <tr className="border-b bg-muted/40 text-muted-foreground font-medium">
                      <th className="py-3 px-4">任务名称 / 下载地址</th>
                      <th className="py-3 px-4">调度账号</th>
                      <th className="py-3 px-4">下载进度</th>
                      <th className="py-3 px-4">任务状态</th>
                      <th className="py-3 px-4">提交时间</th>
                      <th className="py-3 px-4 text-right">操作</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-border/50">
                    {tasks.map((task) => (
                      <tr key={task.id} className="hover:bg-muted/20 transition-colors">
                        <td className="py-3 px-4 max-w-[280px]">
                          <div className="font-semibold truncate text-foreground">
                            {task.file_name || "未命名离线任务"}
                          </div>
                          <div className="text-[11px] font-mono text-muted-foreground/80 truncate">
                            {task.source_url}
                          </div>
                        </td>
                        <td className="py-3 px-4 whitespace-nowrap">
                          <span className="font-medium">{task.account_name || `Account #${task.account_id}`}</span>
                        </td>
                        <td className="py-3 px-4 whitespace-nowrap min-w-[120px]">
                          <div className="flex items-center gap-2">
                            <div className="h-1.5 w-20 rounded-full bg-secondary overflow-hidden">
                              <div
                                className={`h-full rounded-full transition-all duration-300 ${
                                  task.status === "COMPLETE"
                                    ? "bg-emerald-500"
                                    : task.status === "ERROR"
                                    ? "bg-destructive"
                                    : "bg-primary"
                                }`}
                                style={{ width: `${task.progress}%` }}
                              />
                            </div>
                            <span className="font-mono text-[11px] font-semibold">{task.progress}%</span>
                          </div>
                        </td>
                        <td className="py-3 px-4 whitespace-nowrap">
                          {getStatusBadge(task.status)}
                          {task.error_message && (
                            <div className="text-[10px] text-destructive max-w-[150px] truncate mt-0.5" title={task.error_message}>
                              {task.error_message}
                            </div>
                          )}
                        </td>
                        <td className="py-3 px-4 whitespace-nowrap text-muted-foreground">
                          {formatDate(task.created_at)}
                        </td>
                        <td className="py-3 px-4 whitespace-nowrap text-right space-x-1">
                          {task.status === "RUNNING" && (
                            <button
                              onClick={() => handleCancelTask(task.id)}
                              className="p-1 rounded-lg hover:bg-secondary text-muted-foreground hover:text-foreground"
                              title="取消任务"
                            >
                              <RotateCcw className="h-3.5 w-3.5" />
                            </button>
                          )}
                          {task.status === "ERROR" && (
                            <button
                              onClick={() => handleRetryTask(task.id)}
                              className="p-1 rounded-lg hover:bg-secondary text-primary"
                              title="重试任务"
                            >
                              <RefreshCw className="h-3.5 w-3.5" />
                            </button>
                          )}
                          <button
                            onClick={() => handleDeleteTask(task.id)}
                            className="p-1 rounded-lg hover:bg-secondary text-muted-foreground hover:text-destructive"
                            title="删除记录"
                          >
                            <Trash2 className="h-3.5 w-3.5" />
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </div>
      )}

      {/* Policy & Auto Cleanup Tab */}
      {isAdmin && activeTab === "policy" && (
        <div className="space-y-4 w-full">
          {settingsMsg && (
            <div
              className={`rounded-2xl p-4 text-xs sm:text-sm flex items-center gap-2.5 ${
                settingsMsg.isError
                  ? "bg-destructive/15 text-destructive border border-destructive/20"
                  : "bg-emerald-500/15 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20"
              }`}
            >
              {settingsMsg.isError ? <AlertCircle className="h-4 w-4 shrink-0" /> : <CheckCircle2 className="h-4 w-4 shrink-0" />}
              <span>{settingsMsg.text}</span>
            </div>
          )}

          <form onSubmit={handleSaveSettings} className="space-y-4">
            {/* Storage Auto Balancing Card */}
            <div className="rounded-2xl border bg-card p-5 shadow-sm space-y-4">
              <div className="flex items-center justify-between">
                <div>
                  <h3 className="font-semibold text-sm sm:text-base">存储自动均衡 (Storage Balancing)</h3>
                  <p className="text-xs text-muted-foreground mt-0.5">
                    离线下载时优先调度至剩余空间最大的账号，防止特定账号空间溢出
                  </p>
                </div>
                <label className="relative inline-flex items-center cursor-pointer">
                  <input
                    type="checkbox"
                    checked={settings.storage_balancing_enabled}
                    onChange={(e) => setSettings({ ...settings, storage_balancing_enabled: e.target.checked })}
                    className="sr-only peer"
                  />
                  <div className="w-11 h-6 bg-muted peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-primary"></div>
                </label>
              </div>

              {settings.storage_balancing_enabled && (
                <div className="rounded-xl bg-secondary/50 p-3.5 space-y-2 text-xs">
                  <label className="font-medium text-foreground block">
                    账号最小保留空闲空间 (GB)
                  </label>
                  <div className="flex items-center gap-2 max-w-xs">
                    <input
                      type="number"
                      min="1"
                      max="1000"
                      value={settings.storage_min_free_gb}
                      onChange={(e) => setSettings({ ...settings, storage_min_free_gb: parseInt(e.target.value) || 10 })}
                      className="w-full rounded-xl border bg-background px-3 py-1.5 text-xs focus:outline-none focus:ring-2 focus:ring-primary"
                    />
                    <span className="text-xs font-semibold text-muted-foreground shrink-0">GB</span>
                  </div>
                  <p className="text-[11px] text-muted-foreground leading-relaxed">
                    当账号剩余空闲空间低于该阈值时，除非显式指定任务大小且该账号能装下，否则优先跳过此账号。
                  </p>
                </div>
              )}
            </div>

            {/* Auto Cleanup Card */}
            <div className="rounded-2xl border bg-card p-5 shadow-sm space-y-4">
              <div className="flex items-center justify-between">
                <div>
                  <h3 className="font-semibold text-sm sm:text-base">自动清理过期离线文件 (Auto Cleanup)</h3>
                  <p className="text-xs text-muted-foreground mt-0.5">
                    定时或自动清理已完成超过 X 天的离线任务及云端文件，保证账号池容量
                  </p>
                </div>
                <label className="relative inline-flex items-center cursor-pointer">
                  <input
                    type="checkbox"
                    checked={settings.auto_cleanup_enabled}
                    onChange={(e) => setSettings({ ...settings, auto_cleanup_enabled: e.target.checked })}
                    className="sr-only peer"
                  />
                  <div className="w-11 h-6 bg-muted peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-primary"></div>
                </label>
              </div>

              {settings.auto_cleanup_enabled && (
                <div className="rounded-xl bg-secondary/50 p-3.5 space-y-2 text-xs">
                  <label className="font-medium text-foreground block">
                    清理保留周期 (天数)
                  </label>
                  <div className="flex items-center gap-2 max-w-xs">
                    <input
                      type="number"
                      min="1"
                      max="365"
                      value={settings.auto_cleanup_days}
                      onChange={(e) => setSettings({ ...settings, auto_cleanup_days: parseInt(e.target.value) || 7 })}
                      className="w-full rounded-xl border bg-background px-3 py-1.5 text-xs focus:outline-none focus:ring-2 focus:ring-primary"
                    />
                    <span className="text-xs font-semibold text-muted-foreground shrink-0">天前离线的文件</span>
                  </div>
                  <p className="text-[11px] text-muted-foreground leading-relaxed">
                    系统将永久删除已完成超过此天数的文件并清空 PikPak 回收站，自动释放空间。
                  </p>
                </div>
              )}
            </div>

            <div className="flex items-center justify-between pt-1">
              <button
                type="button"
                onClick={handleTriggerCleanupNow}
                disabled={cleaningNow}
                className="flex items-center gap-1.5 rounded-xl border border-destructive/30 px-4 py-2 text-xs font-semibold text-destructive hover:bg-destructive/10 transition-colors"
              >
                {cleaningNow ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Eraser className="h-3.5 w-3.5" />}
                立即手动执行一次清理
              </button>

              <button
                type="submit"
                disabled={savingSettings}
                className="flex items-center gap-2 rounded-xl bg-primary px-5 py-2 text-xs font-semibold text-primary-foreground shadow hover:bg-primary/90 disabled:opacity-50 transition-all"
              >
                {savingSettings && <Loader2 className="h-3.5 w-3.5 animate-spin" />}
                保存策略设置
              </button>
            </div>
          </form>

          {cleanupFeedback && (
            <div
              className={`rounded-2xl p-4 text-xs sm:text-sm flex items-center gap-2.5 ${
                cleanupFeedback.isError
                  ? "bg-destructive/15 text-destructive border border-destructive/20"
                  : "bg-emerald-500/15 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20"
              }`}
            >
              {cleanupFeedback.isError ? <AlertCircle className="h-4 w-4 shrink-0" /> : <CheckCircle2 className="h-4 w-4 shrink-0" />}
              <span>{cleanupFeedback.message}</span>
            </div>
          )}
        </div>
      )}

      {/* Account Management Tab */}
      {isAdmin && activeTab === "accounts" && <AccountsPage />}

      {/* Users Management Tab */}
      {isAdmin && activeTab === "users" && (
        <div className="space-y-4 animate-in fade-in duration-150">
          <div className="flex items-center justify-between rounded-2xl border bg-card p-4 sm:p-5 shadow-sm">
            <div>
              <h2 className="text-base sm:text-lg font-bold">用户管理</h2>
              <p className="text-xs text-muted-foreground mt-0.5">
                添加与管理系统用户，不同用户间的文件和离线任务数据完全相互隔离
              </p>
            </div>
            <button
              onClick={() => {
                setAddUserError("")
                setShowAddUserModal(true)
              }}
              className="flex items-center gap-1.5 rounded-xl bg-primary px-3.5 py-2 text-xs sm:text-sm font-medium text-primary-foreground shadow hover:bg-primary/90 transition-colors"
            >
              <UserPlus className="h-4 w-4" />
              <span>添加用户</span>
            </button>
          </div>

          <div className="rounded-2xl border bg-card shadow-sm overflow-hidden">
            {loadingUsers ? (
              <div className="flex items-center justify-center p-12 text-muted-foreground gap-2">
                <Loader2 className="h-5 w-5 animate-spin text-primary" />
                <span className="text-xs font-medium">正在加载用户列表...</span>
              </div>
            ) : userList.length === 0 ? (
              <div className="p-12 text-center text-xs text-muted-foreground">暂无其他用户数据</div>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-left text-xs border-collapse">
                  <thead>
                    <tr className="border-b bg-muted/40 text-muted-foreground font-medium">
                      <th className="py-3 px-4">用户名</th>
                      <th className="py-3 px-4">角色权限</th>
                      <th className="py-3 px-4">创建时间</th>
                      <th className="py-3 px-4 text-right">操作</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-border/50">
                    {userList.map((u) => (
                      <tr key={u.id} className="hover:bg-muted/20 transition-colors">
                        <td className="py-3 px-4 font-semibold text-foreground flex items-center gap-2">
                          <div className="h-7 w-7 rounded-lg bg-primary/15 text-primary flex items-center justify-center font-bold text-xs uppercase">
                            {u.username.slice(0, 1)}
                          </div>
                          <span>{u.username}</span>
                          {u.username === currentUsername && (
                            <span className="text-[10px] bg-secondary px-1.5 py-0.5 rounded text-muted-foreground">
                              当前账号
                            </span>
                          )}
                        </td>
                        <td className="py-3 px-4">
                          {u.role === "admin" ? (
                            <span className="inline-flex items-center rounded-full bg-purple-500/15 text-purple-600 dark:text-purple-400 px-2 py-0.5 text-[11px] font-medium">
                              管理员
                            </span>
                          ) : (
                            <span className="inline-flex items-center rounded-full bg-emerald-500/15 text-emerald-600 dark:text-emerald-400 px-2 py-0.5 text-[11px] font-medium">
                              普通用户 (空间隔离)
                            </span>
                          )}
                        </td>
                        <td className="py-3 px-4 text-muted-foreground">
                          {formatDate(u.created_at)}
                        </td>
                        <td className="py-3 px-4 text-right space-x-2">
                          <button
                            onClick={() => {
                              setResetUser(u)
                              setResetPassword("")
                              setResetError("")
                            }}
                            className="rounded-lg border px-2.5 py-1 text-xs hover:bg-secondary text-muted-foreground hover:text-foreground transition-colors"
                          >
                            重置密码
                          </button>
                          <button
                            onClick={() => handleDeleteUser(u)}
                            disabled={u.username === currentUsername}
                            className="rounded-lg border border-destructive/30 px-2.5 py-1 text-xs text-destructive hover:bg-destructive/10 disabled:opacity-30 transition-colors"
                          >
                            删除
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>

          {/* Add User Modal */}
          {showAddUserModal && (
            <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4 animate-in fade-in duration-150">
              <div className="flex flex-col w-full max-w-md rounded-2xl border bg-card p-6 shadow-2xl animate-in zoom-in-95">
                <div className="flex items-center justify-between border-b pb-3 mb-4">
                  <div className="flex items-center gap-2 font-semibold text-base text-foreground">
                    <UserPlus className="h-5 w-5 text-primary" />
                    添加新用户
                  </div>
                  <button
                    onClick={() => setShowAddUserModal(false)}
                    className="flex h-8 w-8 items-center justify-center rounded-xl text-muted-foreground hover:text-foreground hover:bg-secondary transition-colors"
                  >
                    <X className="h-4 w-4" />
                  </button>
                </div>

                {addUserError && (
                  <div className="mb-4 flex items-center gap-2 rounded-xl bg-destructive/15 p-3 text-xs text-destructive">
                    <AlertCircle className="h-4 w-4 shrink-0" />
                    <span>{addUserError}</span>
                  </div>
                )}

                <form onSubmit={handleCreateUser} className="space-y-4">
                  <div>
                    <label className="text-xs font-semibold text-muted-foreground">用户名</label>
                    <input
                      type="text"
                      required
                      autoFocus
                      placeholder="至少2个字符，仅限英文字母/数字"
                      value={addUsername}
                      onChange={(e) => setAddUsername(e.target.value)}
                      className="mt-1.5 w-full rounded-xl border bg-background px-3 py-2 text-xs focus:outline-none focus:ring-2 focus:ring-primary"
                    />
                  </div>

                  <div>
                    <label className="text-xs font-semibold text-muted-foreground">初始密码</label>
                    <input
                      type="password"
                      required
                      placeholder="至少6位密码"
                      value={addPassword}
                      onChange={(e) => setAddPassword(e.target.value)}
                      className="mt-1.5 w-full rounded-xl border bg-background px-3 py-2 text-xs focus:outline-none focus:ring-2 focus:ring-primary"
                    />
                  </div>

                  <div>
                    <label className="text-xs font-semibold text-muted-foreground">用户角色</label>
                    <select
                      value={addRole}
                      onChange={(e) => setAddRole(e.target.value)}
                      className="mt-1.5 w-full rounded-xl border bg-background px-3 py-2 text-xs focus:outline-none focus:ring-2 focus:ring-primary"
                    >
                      <option value="user">普通用户 (仅查看与操作自身文件，无权访问管理设置)</option>
                      <option value="admin">系统管理员 (拥有完整多账号与系统管理权限)</option>
                    </select>
                  </div>

                  <div className="flex items-center justify-end gap-2.5 pt-2">
                    <button
                      type="button"
                      onClick={() => setShowAddUserModal(false)}
                      className="rounded-xl border px-4 py-2 text-xs font-medium hover:bg-secondary"
                    >
                      取消
                    </button>
                    <button
                      type="submit"
                      disabled={addingUser || !addUsername.trim() || !addPassword}
                      className="flex items-center gap-2 rounded-xl bg-primary px-5 py-2 text-xs font-medium text-primary-foreground shadow hover:bg-primary/90 disabled:opacity-50"
                    >
                      {addingUser && <Loader2 className="h-3.5 w-3.5 animate-spin" />}
                      确认添加
                    </button>
                  </div>
                </form>
              </div>
            </div>
          )}

          {/* Reset Password Modal */}
          {resetUser && (
            <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4 animate-in fade-in duration-150">
              <div className="flex flex-col w-full max-w-md rounded-2xl border bg-card p-6 shadow-2xl animate-in zoom-in-95">
                <div className="flex items-center justify-between border-b pb-3 mb-4">
                  <div className="flex items-center gap-2 font-semibold text-base text-foreground">
                    <Lock className="h-5 w-5 text-primary" />
                    重置用户密码
                  </div>
                  <button
                    onClick={() => setResetUser(null)}
                    className="flex h-8 w-8 items-center justify-center rounded-xl text-muted-foreground hover:text-foreground hover:bg-secondary transition-colors"
                  >
                    <X className="h-4 w-4" />
                  </button>
                </div>

                {resetError && (
                  <div className="mb-4 flex items-center gap-2 rounded-xl bg-destructive/15 p-3 text-xs text-destructive">
                    <AlertCircle className="h-4 w-4 shrink-0" />
                    <span>{resetError}</span>
                  </div>
                )}

                <form onSubmit={handleResetPassword} className="space-y-4">
                  <div>
                    <p className="text-xs text-muted-foreground mb-3">
                      正在为用户 <span className="font-semibold text-foreground font-mono">{resetUser.username}</span> 设置新密码：
                    </p>
                    <label className="text-xs font-semibold text-muted-foreground">新密码</label>
                    <input
                      type="password"
                      required
                      autoFocus
                      placeholder="输入至少6位新密码"
                      value={resetPassword}
                      onChange={(e) => setResetPassword(e.target.value)}
                      className="mt-1.5 w-full rounded-xl border bg-background px-3 py-2 text-xs focus:outline-none focus:ring-2 focus:ring-primary"
                    />
                  </div>

                  <div className="flex items-center justify-end gap-2.5 pt-2">
                    <button
                      type="button"
                      onClick={() => setResetUser(null)}
                      className="rounded-xl border px-4 py-2 text-xs font-medium hover:bg-secondary"
                    >
                      取消
                    </button>
                    <button
                      type="submit"
                      disabled={resettingPassword || !resetPassword}
                      className="flex items-center gap-2 rounded-xl bg-primary px-5 py-2 text-xs font-medium text-primary-foreground shadow hover:bg-primary/90 disabled:opacity-50"
                    >
                      {resettingPassword && <Loader2 className="h-3.5 w-3.5 animate-spin" />}
                      确认重置
                    </button>
                  </div>
                </form>
              </div>
            </div>
          )}
        </div>
      )}

      {/* API Keys Tab */}
      {isAdmin && activeTab === "apikeys" && <ApiKeysPage />}

      {/* Profile / Admin Info Tab */}
      {activeTab === "profile" && (
        <div className="space-y-4 w-full animate-in fade-in duration-150">
          {profileMsg && (
            <div
              className={`rounded-xl p-3 text-xs flex items-center gap-2 ${
                profileMsg.isError
                  ? "bg-destructive/15 text-destructive border border-destructive/20"
                  : "bg-emerald-500/15 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20"
              }`}
            >
              {profileMsg.isError ? <AlertCircle className="h-4 w-4 shrink-0" /> : <Check className="h-4 w-4 shrink-0" />}
              <span>{profileMsg.text}</span>
            </div>
          )}

          <form onSubmit={handleSaveProfile} className="space-y-4">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              {/* Username Card */}
              <div className="rounded-2xl border bg-card p-5 shadow-sm space-y-3">
                <div className="font-semibold text-sm flex items-center gap-2">
                  <User className="h-4 w-4 text-primary" />
                  {isAdmin ? "管理员账号" : "个人账号"}
                </div>

                <div>
                  <label className="text-xs font-semibold text-muted-foreground">登录用户名 *</label>
                  <input
                    type="text"
                    required
                    value={newUsername}
                    onChange={(e) => setNewUsername(e.target.value)}
                    className="mt-1.5 w-full rounded-xl border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
                    placeholder="username"
                  />
                  <span className="text-[11px] text-muted-foreground mt-1 block">
                    {isAdmin ? "用于后台管理面板登录认证" : "用于个人登录认证"}
                  </span>
                </div>
              </div>

              {/* Password Security Card */}
              <div className="rounded-2xl border bg-card p-5 shadow-sm space-y-3">
                <div className="font-semibold text-sm flex items-center gap-2">
                  <Lock className="h-4 w-4 text-primary" />
                  修改登录密码
                </div>

                <div>
                  <label className="text-xs font-semibold text-muted-foreground">当前原密码</label>
                  <input
                    type="password"
                    value={oldPassword}
                    onChange={(e) => setOldPassword(e.target.value)}
                    className="mt-1 w-full rounded-xl border bg-background px-3 py-1.5 text-xs focus:outline-none focus:ring-2 focus:ring-primary"
                    placeholder="若修改密码则必填"
                  />
                </div>

                <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
                  <div>
                    <label className="text-xs font-semibold text-muted-foreground">新密码</label>
                    <input
                      type="password"
                      value={newPassword}
                      onChange={(e) => setNewPassword(e.target.value)}
                      className="mt-1 w-full rounded-xl border bg-background px-3 py-1.5 text-xs focus:outline-none focus:ring-2 focus:ring-primary"
                      placeholder="至少6位"
                    />
                  </div>
                  <div>
                    <label className="text-xs font-semibold text-muted-foreground">确认新密码</label>
                    <input
                      type="password"
                      value={confirmPassword}
                      onChange={(e) => setConfirmPassword(e.target.value)}
                      className="mt-1 w-full rounded-xl border bg-background px-3 py-1.5 text-xs focus:outline-none focus:ring-2 focus:ring-primary"
                      placeholder="再次输入新密码"
                    />
                  </div>
                </div>
              </div>
            </div>

            <div className="flex justify-end pt-1">
              <button
                type="submit"
                disabled={profileSaving}
                className="flex items-center gap-2 rounded-xl bg-primary px-5 py-2 text-xs font-semibold text-primary-foreground shadow hover:bg-primary/90 disabled:opacity-50 transition-all"
              >
                {profileSaving && <Loader2 className="h-3.5 w-3.5 animate-spin" />}
                {isAdmin ? "保存管理员信息" : "保存个人设置"}
              </button>
            </div>
          </form>
        </div>
      )}

      {/* Audit Log Tab */}
      {isAdmin && activeTab === "audit" && <AuditPage />}

      {/* About Tab */}
      {activeTab === "about" && (
        <div className="space-y-6 w-full animate-in fade-in duration-150">
          <div className="rounded-3xl border bg-card p-6 sm:p-8 shadow-sm space-y-6">
            <div className="flex flex-col sm:flex-row sm:items-center gap-5 pb-6 border-b">
              <img
                src="/logo.png"
                alt="PikLeech Logo"
                className="w-16 h-16 sm:w-20 sm:h-20 rounded-2xl shadow-md object-cover"
              />
              <div className="space-y-1.5">
                <div className="flex items-center gap-3">
                  <h2 className="text-2xl font-black tracking-tight">PikLeech</h2>
                  <span className="rounded-full bg-primary/15 px-2.5 py-0.5 text-xs font-semibold text-primary">
                    v1.0.0 Stable
                  </span>
                </div>
                <p className="text-sm text-muted-foreground">
                  高性能 PikPak 多账号聚合离线下载与虚拟云盘管理系统
                </p>
              </div>
            </div>

            {/* Description */}
            <div className="space-y-3">
              <h3 className="text-sm font-bold tracking-tight">项目简介</h3>
              <p className="text-xs sm:text-sm text-muted-foreground leading-relaxed">
                PikLeech 专为多 PikPak 账号用户设计，将多个独立账号智能聚合成统一网盘。
                系统支持每日离线额度智能轮询与故障转移、存储空间动态负载均衡、多用户物理级隔离管理、
                磁力与直链高速离线调度，并提供原生直链解析与在线无缝视频点播体验。
              </p>
            </div>

            {/* Features list */}
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 pt-2">
              <div className="flex items-start gap-3 rounded-2xl border p-4 bg-secondary/20">
                <DownloadCloud className="h-5 w-5 text-primary shrink-0 mt-0.5" />
                <div>
                  <h4 className="text-xs font-bold">智能额度轮询</h4>
                  <p className="text-[11px] text-muted-foreground mt-0.5">单账号配额用尽或受限时，无缝切换下一个可用账号继续下载</p>
                </div>
              </div>
              <div className="flex items-start gap-3 rounded-2xl border p-4 bg-secondary/20">
                <HardDrive className="h-5 w-5 text-primary shrink-0 mt-0.5" />
                <div>
                  <h4 className="text-xs font-bold">存储自动均衡</h4>
                  <p className="text-[11px] text-muted-foreground mt-0.5">智能评估剩余容量与文件大小，防止单个账号爆满，支持过期自动清理</p>
                </div>
              </div>
              <div className="flex items-start gap-3 rounded-2xl border p-4 bg-secondary/20">
                <Users className="h-5 w-5 text-primary shrink-0 mt-0.5" />
                <div>
                  <h4 className="text-xs font-bold">多用户严格隔离</h4>
                  <p className="text-[11px] text-muted-foreground mt-0.5">普通用户与管理员物理级目录隔离，任务与文件互不可见</p>
                </div>
              </div>
              <div className="flex items-start gap-3 rounded-2xl border p-4 bg-secondary/20">
                <Server className="h-5 w-5 text-primary shrink-0 mt-0.5" />
                <div>
                  <h4 className="text-xs font-bold">直链流媒体播放</h4>
                  <p className="text-[11px] text-muted-foreground mt-0.5">多码率视频直链点播，支持 Range 206 毫秒级分片拖拽与代理备选</p>
                </div>
              </div>
            </div>

            {/* Project repository card */}
            <div className="rounded-2xl border p-5 bg-secondary/30 flex flex-col sm:flex-row sm:items-center justify-between gap-4">
              <div className="space-y-1">
                <div className="flex items-center gap-2 text-xs font-bold">
                  <Github className="h-4 w-4" />
                  <span>官方开源仓库</span>
                </div>
                <div className="text-xs text-muted-foreground font-mono select-all">
                  https://github.com/akasls/PikLeech
                </div>
              </div>
              <div className="flex items-center gap-2">
                <button
                  type="button"
                  onClick={() => {
                    navigator.clipboard.writeText("https://github.com/akasls/PikLeech")
                    setCopiedRepo(true)
                    setTimeout(() => setCopiedRepo(false), 2000)
                  }}
                  className="flex items-center gap-1.5 rounded-xl border bg-background px-3 py-1.5 text-xs font-medium hover:bg-secondary/80 transition-colors"
                >
                  {copiedRepo ? <Check className="h-3.5 w-3.5 text-emerald-500" /> : <Copy className="h-3.5 w-3.5" />}
                  <span>{copiedRepo ? "已复制" : "复制地址"}</span>
                </button>
                <a
                  href="https://github.com/akasls/PikLeech"
                  target="_blank"
                  rel="noreferrer"
                  className="flex items-center gap-1.5 rounded-xl bg-primary px-3.5 py-1.5 text-xs font-semibold text-primary-foreground shadow-sm hover:bg-primary/90 transition-colors"
                >
                  <ExternalLink className="h-3.5 w-3.5" />
                  <span>访问 GitHub</span>
                </a>
              </div>
            </div>

            {/* System Info & License */}
            <div className="pt-2 text-[11px] text-muted-foreground flex flex-wrap items-center justify-between gap-2 border-t">
              <span>架构：Go 1.22 + SQLite WAL + React 18 + Tailwind CSS</span>
              <span>MIT License · 开源免费</span>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
