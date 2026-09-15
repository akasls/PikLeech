import React, { useState, useEffect } from "react"
import {
  Server,
  Plus,
  RefreshCw,
  Edit2,
  Trash2,
  RotateCcw,
  CheckCircle2,
  AlertTriangle,
  XCircle,
  Clock,
  ShieldCheck,
  Globe,
  Loader2,
  Play,
  X,
  Eraser,
} from "lucide-react"
import { api, Account } from "../lib/api"
import { formatBytes, formatDate } from "../lib/utils"

export const AccountsPage: React.FC = () => {
  const [accounts, setAccounts] = useState<Account[]>([])
  const [loading, setLoading] = useState(true)

  // Modals
  const [showAddModal, setShowAddModal] = useState(false)
  const [editingAccount, setEditingAccount] = useState<Account | null>(null)
  const [testingID, setTestingID] = useState<number | null>(null)
  const [testResult, setTestResult] = useState<{ id: number; data: any } | null>(null)
  const [initializingID, setInitializingID] = useState<number | null>(null)
  const [initFeedback, setInitFeedback] = useState<{ id: number; message: string; isError?: boolean } | null>(null)

  // Standalone Proxy Test Modal
  const [showProxyTestModal, setShowProxyTestModal] = useState(false)
  const [testProxyURL, setTestProxyURL] = useState("")
  const [proxyTesting, setProxyTesting] = useState(false)
  const [proxyTestResult, setProxyTestResult] = useState<any>(null)

  const handleInitializeAccount = async (id: number, name: string) => {
    setInitializingID(id)
    setInitFeedback(null)
    try {
      await api.initializeAccount(id)
      setInitFeedback({ id, message: `账号「${name}」已完成初始化：远端文件与离线任务已清空，容量已重置。` })
      loadAccounts()
    } catch (err: any) {
      setInitFeedback({ id, message: `初始化失败: ${err.message}`, isError: true })
    } finally {
      setInitializingID(null)
    }
  }

  const loadAccounts = async () => {
    setLoading(true)
    try {
      const data = await api.listAccounts()
      setAccounts(data || [])
    } catch (err) {
      console.error(err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadAccounts()
  }, [])

  const handleToggleEnable = async (acc: Account) => {
    try {
      await api.updateAccount(acc.id, { is_enabled: !acc.is_enabled })
      loadAccounts()
    } catch (err: any) {
      alert("更新失败: " + err.message)
    }
  }

  const handleTestAccount = async (id: number) => {
    setTestingID(id)
    setTestResult(null)
    try {
      const res = await api.testAccount(id)
      setTestResult({ id, data: res })
      loadAccounts()
    } catch (err: any) {
      setTestResult({ id, data: { success: false, error: err.message } })
    } finally {
      setTestingID(null)
    }
  }

  const handleResetQuota = async (id: number) => {
    try {
      await api.resetQuota(id)
      loadAccounts()
    } catch (err: any) {
      alert("重置额度失败: " + err.message)
    }
  }

  const handleDelete = async (id: number, name: string) => {
    if (!confirm(`确定要删除账号 [${name}] 吗？`)) return
    try {
      await api.deleteAccount(id)
      loadAccounts()
    } catch (err: any) {
      alert("删除失败: " + err.message)
    }
  }

  const handleTestProxyStandalone = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!testProxyURL.trim()) return
    setProxyTesting(true)
    setProxyTestResult(null)
    try {
      const res = await api.testProxy(testProxyURL.trim())
      setProxyTestResult(res)
    } catch (err: any) {
      setProxyTestResult({ success: false, error: err.message })
    } finally {
      setProxyTesting(false)
    }
  }

  const getStatusBadge = (acc: Account) => {
    if (!acc.is_enabled) {
      return (
        <span className="inline-flex items-center gap-1 rounded-full bg-secondary px-2.5 py-0.5 text-xs font-medium text-muted-foreground">
          <XCircle className="h-3.5 w-3.5" />
          已禁用
        </span>
      )
    }

    switch (acc.status) {
      case "HEALTHY":
        return (
          <span className="inline-flex items-center gap-1 rounded-full bg-emerald-500/15 px-2.5 py-0.5 text-xs font-medium text-emerald-600 dark:text-emerald-400">
            <CheckCircle2 className="h-3.5 w-3.5" />
            正常
          </span>
        )
      case "QUOTA_EXHAUSTED":
        return (
          <span className="inline-flex items-center gap-1 rounded-full bg-amber-500/15 px-2.5 py-0.5 text-xs font-medium text-amber-600 dark:text-amber-400">
            <AlertTriangle className="h-3.5 w-3.5" />
            今日额度已耗尽
          </span>
        )
      case "AUTH_FAILED":
        return (
          <span className="inline-flex items-center gap-1 rounded-full bg-destructive/15 px-2.5 py-0.5 text-xs font-medium text-destructive">
            <XCircle className="h-3.5 w-3.5" />
            登录失效
          </span>
        )
      case "PROXY_FAILED":
        return (
          <span className="inline-flex items-center gap-1 rounded-full bg-orange-500/15 px-2.5 py-0.5 text-xs font-medium text-orange-600 dark:text-orange-400">
            <Globe className="h-3.5 w-3.5" />
            代理异常
          </span>
        )
      case "RATE_LIMITED":
      case "COOLDOWN":
        return (
          <span className="inline-flex items-center gap-1 rounded-full bg-blue-500/15 px-2.5 py-0.5 text-xs font-medium text-blue-600 dark:text-blue-400">
            <Clock className="h-3.5 w-3.5" />
            冷却中
          </span>
        )
      default:
        return (
          <span className="inline-flex items-center gap-1 rounded-full bg-secondary px-2.5 py-0.5 text-xs font-medium text-muted-foreground">
            {acc.status}
          </span>
        )
    }
  }

  return (
    <div className="space-y-4">
      {/* Action Bar */}
      <div className="flex items-center justify-between border-b pb-3.5">
        <h2 className="text-lg font-bold tracking-tight">PikPak 多账号管理</h2>

        <div className="flex items-center gap-2">
          <button
            onClick={loadAccounts}
            className="p-2 rounded-xl border hover:bg-secondary text-muted-foreground hover:text-foreground"
            title="刷新"
          >
            <RefreshCw className={`h-4 w-4 ${loading ? "animate-spin" : ""}`} />
          </button>

          <button
            onClick={() => setShowAddModal(true)}
            className="flex items-center gap-1.5 rounded-xl bg-primary px-3.5 py-2 text-xs font-medium text-primary-foreground shadow hover:bg-primary/90 transition-all"
          >
            <Plus className="h-4 w-4" />
            添加 PikPak 账号
          </button>
        </div>
      </div>

      {/* Accounts List / Cards */}
      {loading && accounts.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 text-muted-foreground gap-3">
          <Loader2 className="h-8 w-8 animate-spin text-primary" />
          <span className="text-sm">正在加载账号列表...</span>
        </div>
      ) : accounts.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 text-muted-foreground">
          <Server className="h-16 w-16 stroke-1 text-muted-foreground/40 mb-3" />
          <p className="text-base font-semibold text-foreground">暂无 PikPak 账号</p>
          <p className="text-xs text-muted-foreground mt-1">
            点击右上角“添加 PikPak 账号”，输入账号密码或 Refresh Token 即可聚合使用！
          </p>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {accounts.map((acc) => {
            const isTestingThis = testingID === acc.id
            return (
              <div
                key={acc.id}
                className="flex flex-col justify-between rounded-2xl border bg-card p-5 shadow-sm hover:border-primary/40 transition-all"
              >
                <div>
                  {/* Title & Status */}
                  <div className="flex items-start justify-between gap-2">
                    <div>
                      <h3 className="font-semibold text-base">
                        {acc.name}
                      </h3>
                      <p className="text-xs text-muted-foreground mt-0.5 font-mono">
                        {acc.username || "Token 授权模式"}
                      </p>
                    </div>
                    {getStatusBadge(acc)}
                  </div>

                  {/* Details Grid */}
                  <div className="mt-4 space-y-2 text-xs">
                    <div className="flex items-center justify-between border-b pb-1.5">
                      <span className="text-muted-foreground">独立代理:</span>
                      <span className="font-mono text-foreground truncate max-w-[180px]" title={acc.proxy_url}>
                        {acc.proxy_url || "直连 (无代理)"}
                      </span>
                    </div>

                    <div className="flex items-center justify-between border-b pb-1.5">
                      <span className="text-muted-foreground">今日已用离线任务:</span>
                      <span className="font-semibold text-foreground">{acc.daily_task_count} 次</span>
                    </div>

                    <div className="flex items-center justify-between border-b pb-1.5">
                      <span className="text-muted-foreground">占用空间 / 容量:</span>
                      <span className="text-foreground">
                        {formatBytes(acc.used_space)} / {formatBytes(acc.total_space)}
                      </span>
                    </div>

                    <div className="flex items-center justify-between text-muted-foreground">
                      <span>最后成功连接:</span>
                      <span>{formatDate(acc.last_success_at)}</span>
                    </div>

                    {acc.last_error && (
                      <div className="rounded-lg bg-destructive/10 p-2 text-[11px] text-destructive leading-tight">
                        最近错误: {acc.last_error}
                      </div>
                    )}
                  </div>

                  {/* In-card test result feedback */}
                  {testResult && testResult.id === acc.id && (
                    <div
                      className={`mt-3 rounded-lg p-2.5 text-xs ${
                        testResult.data.success
                          ? "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400"
                          : "bg-destructive/10 text-destructive"
                      }`}
                    >
                      {testResult.data.success
                        ? `测试成功！延迟: ${testResult.data.latency_ms}ms · 已更新空间配额`
                        : `测试失败: ${testResult.data.error || "未知异常"}`}
                    </div>
                  )}

                  {/* In-card initialization feedback */}
                  {initFeedback && initFeedback.id === acc.id && (
                    <div
                      className={`mt-3 rounded-lg p-2.5 text-xs ${
                        !initFeedback.isError
                          ? "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400"
                          : "bg-destructive/10 text-destructive"
                      }`}
                    >
                      {initFeedback.message}
                    </div>
                  )}
                </div>

                {/* Card Action Buttons */}
                <div className="mt-5 flex items-center justify-between border-t pt-3.5">
                  <div className="flex items-center gap-1.5 flex-wrap">
                    <button
                      onClick={() => handleTestAccount(acc.id)}
                      disabled={isTestingThis}
                      className="flex items-center gap-1 rounded-lg border bg-secondary/60 px-2.5 py-1 text-xs font-medium hover:bg-secondary disabled:opacity-50"
                      title="手动测试连通性与刷新 Token"
                    >
                      {isTestingThis ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Play className="h-3.5 w-3.5" />}
                      测试账号
                    </button>

                    <button
                      onClick={() => handleInitializeAccount(acc.id, acc.name)}
                      disabled={initializingID === acc.id}
                      className="flex items-center gap-1 rounded-lg border border-destructive/30 bg-destructive/10 px-2.5 py-1 text-xs font-medium text-destructive hover:bg-destructive/20 disabled:opacity-50"
                      title="一键清空该账号在 PikPak 上的所有文件与离线任务，恢复空网盘容量"
                    >
                      {initializingID === acc.id ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Eraser className="h-3.5 w-3.5" />}
                      初始化
                    </button>

                    {acc.status === "QUOTA_EXHAUSTED" && (
                      <button
                        onClick={() => handleResetQuota(acc.id)}
                        className="flex items-center gap-1 rounded-lg border border-amber-500/30 bg-amber-500/10 px-2.5 py-1 text-xs font-medium text-amber-600 dark:text-amber-400 hover:bg-amber-500/20"
                        title="手动恢复该账号可用状态"
                      >
                        <RotateCcw className="h-3.5 w-3.5" />
                        重置额度
                      </button>
                    )}
                  </div>

                  <div className="flex items-center gap-1">
                    <button
                      onClick={() => handleToggleEnable(acc)}
                      className={`px-2.5 py-1 rounded-lg text-xs font-medium transition-colors ${
                        acc.is_enabled
                          ? "bg-secondary text-muted-foreground hover:text-foreground"
                          : "bg-primary text-primary-foreground font-semibold"
                      }`}
                    >
                      {acc.is_enabled ? "停用" : "启用"}
                    </button>
                    <button
                      onClick={() => setEditingAccount(acc)}
                      className="p-1.5 rounded-lg hover:bg-secondary text-muted-foreground hover:text-foreground"
                      title="编辑账号"
                    >
                      <Edit2 className="h-3.5 w-3.5" />
                    </button>
                    <button
                      onClick={() => handleDelete(acc.id, acc.name)}
                      className="p-1.5 rounded-lg hover:bg-destructive/10 text-muted-foreground hover:text-destructive"
                      title="删除账号"
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                    </button>
                  </div>
                </div>
              </div>
            )
          })}
        </div>
      )}

      {/* Add / Edit Account Modal */}
      {(showAddModal || editingAccount) && (
        <AccountFormModal
          account={editingAccount}
          onClose={() => {
            setShowAddModal(false)
            setEditingAccount(null)
          }}
          onSuccess={() => {
            setShowAddModal(false)
            setEditingAccount(null)
            loadAccounts()
          }}
        />
      )}

      {/* Standalone Proxy Test Modal */}
      {showProxyTestModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4">
          <div className="w-full max-w-md rounded-2xl border bg-card p-6 shadow-2xl animate-in zoom-in-95">
            <div className="flex items-center justify-between border-b pb-3 mb-4">
              <h3 className="font-semibold text-base flex items-center gap-2">
                <Globe className="h-4 w-4 text-primary" />
                测试网络代理连通性
              </h3>
              <button onClick={() => setShowProxyTestModal(false)} className="p-1 rounded-lg hover:bg-secondary">
                <X className="h-5 w-5" />
              </button>
            </div>

            <form onSubmit={handleTestProxyStandalone} className="space-y-4">
              <div>
                <label className="text-xs font-semibold text-muted-foreground uppercase">
                  代理 URL (HTTP / HTTPS / SOCKS5 / SOCKS5H)
                </label>
                <input
                  type="text"
                  required
                  placeholder="socks5://127.0.0.1:10808 或 http://proxy.com:8080"
                  value={testProxyURL}
                  onChange={(e) => setTestProxyURL(e.target.value)}
                  className="mt-1.5 w-full rounded-xl border bg-background px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-primary"
                />
              </div>

              {proxyTestResult && (
                <div
                  className={`rounded-xl p-3 text-xs space-y-1 ${
                    proxyTestResult.success
                      ? "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20"
                      : "bg-destructive/10 text-destructive border border-destructive/20"
                  }`}
                >
                  <div className="font-semibold">
                    {proxyTestResult.success ? "代理测试通过！" : "代理连接失败"}
                  </div>
                  {proxyTestResult.success && (
                    <>
                      <div>出口 IP: <span className="font-mono">{proxyTestResult.egress_ip}</span></div>
                      <div>连接延迟: <span className="font-mono">{proxyTestResult.latency_ms} ms</span></div>
                    </>
                  )}
                  {!proxyTestResult.success && <div>原因: {proxyTestResult.error}</div>}
                </div>
              )}

              <div className="flex items-center justify-end gap-3 pt-2">
                <button
                  type="button"
                  onClick={() => setShowProxyTestModal(false)}
                  className="rounded-xl border px-4 py-2 text-sm font-medium hover:bg-secondary"
                >
                  关闭
                </button>
                <button
                  type="submit"
                  disabled={proxyTesting || !testProxyURL.trim()}
                  className="flex items-center gap-2 rounded-xl bg-primary px-5 py-2 text-sm font-medium text-primary-foreground shadow hover:bg-primary/90 disabled:opacity-50"
                >
                  {proxyTesting ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
                  开始测试
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}

interface AccountFormModalProps {
  account: Account | null
  onClose: () => void
  onSuccess: () => void
}

const AccountFormModal: React.FC<AccountFormModalProps> = ({ account, onClose, onSuccess }) => {
  const isEdit = !!account

  const [name, setName] = useState(account?.name || "")
  const [username, setUsername] = useState(account?.username || "")
  const [password, setPassword] = useState("")
  const [refreshToken, setRefreshToken] = useState("")
  const [proxyURL, setProxyURL] = useState(account?.proxy_url || "")
  const [priority, setPriority] = useState(account?.priority || 10)
  const [isEnabled, setIsEnabled] = useState(account ? account.is_enabled : true)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState("")

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setSubmitting(true)
    setError("")

    try {
      if (isEdit) {
        const payload: any = {
          name,
          username,
          proxy_url: proxyURL,
          priority: Number(priority),
          is_enabled: isEnabled,
        }
        if (password.trim()) payload.password = password.trim()
        if (refreshToken.trim()) payload.refresh_token = refreshToken.trim()
        await api.updateAccount(account.id, payload)
      } else {
        await api.createAccount({
          name,
          username,
          password: password.trim(),
          refresh_token: refreshToken.trim(),
          proxy_url: proxyURL.trim(),
          priority: Number(priority),
          is_enabled: isEnabled,
        })
      }
      onSuccess()
    } catch (err: any) {
      setError(err.message || "提交失败")
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4">
      <div className="w-full max-w-lg rounded-2xl border bg-card p-6 shadow-2xl animate-in zoom-in-95 max-h-[90vh] overflow-y-auto">
        <div className="flex items-center justify-between border-b pb-3 mb-4">
          <h3 className="font-semibold text-base flex items-center gap-2">
            <Server className="h-4 w-4 text-primary" />
            {isEdit ? "编辑 PikPak 账号" : "添加 PikPak 账号"}
          </h3>
          <button onClick={onClose} className="p-1 rounded-lg hover:bg-secondary">
            <X className="h-5 w-5" />
          </button>
        </div>

        {error && (
          <div className="mb-4 rounded-xl bg-destructive/15 p-3 text-xs text-destructive">
            {error}
          </div>
        )}

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="text-xs font-semibold text-muted-foreground uppercase">账号备注名称 *</label>
            <input
              type="text"
              required
              placeholder="例如: 美西节点1号 / 香港备用号"
              value={name}
              onChange={(e) => setName(e.target.value)}
              className="mt-1.5 w-full rounded-xl border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
            />
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div>
              <label className="text-xs font-semibold text-muted-foreground uppercase">用户名 / 邮箱 / 手机号</label>
              <input
                type="text"
                placeholder="user@example.com"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                className="mt-1.5 w-full rounded-xl border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
              />
            </div>
            <div>
              <label className="text-xs font-semibold text-muted-foreground uppercase">
                密码 {isEdit ? "(留空不修改)" : ""}
              </label>
              <input
                type="password"
                placeholder="账号密码"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                className="mt-1.5 w-full rounded-xl border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
              />
            </div>
          </div>

          <div>
            <label className="text-xs font-semibold text-muted-foreground uppercase">
              Refresh Token {isEdit ? "(留空不修改)" : "(选填，可免账密登录)"}
            </label>
            <input
              type="password"
              placeholder="PikPak Refresh Token"
              value={refreshToken}
              onChange={(e) => setRefreshToken(e.target.value)}
              className="mt-1.5 w-full rounded-xl border bg-background px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-primary"
            />
          </div>

          <div>
            <label className="text-xs font-semibold text-muted-foreground uppercase">
              独立网络代理 (HTTP / HTTPS / SOCKS5 / SOCKS5H)
            </label>
            <input
              type="text"
              placeholder="留空直连，或 socks5://127.0.0.1:10808"
              value={proxyURL}
              onChange={(e) => setProxyURL(e.target.value)}
              className="mt-1.5 w-full rounded-xl border bg-background px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-primary"
            />
          </div>

          <div className="grid grid-cols-2 gap-3 items-center">
            <div>
              <label className="text-xs font-semibold text-muted-foreground uppercase">调度优先级 (越大越优先)</label>
              <input
                type="number"
                min="1"
                max="100"
                value={priority}
                onChange={(e) => setPriority(parseInt(e.target.value) || 10)}
                className="mt-1.5 w-full rounded-xl border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
              />
            </div>

            <div className="pt-5 flex items-center gap-2">
              <input
                type="checkbox"
                id="enableToggle"
                checked={isEnabled}
                onChange={(e) => setIsEnabled(e.target.checked)}
                className="rounded border-gray-300"
              />
              <label htmlFor="enableToggle" className="text-sm font-medium cursor-pointer">
                立即启用此账号
              </label>
            </div>
          </div>

          <div className="rounded-xl bg-secondary/40 p-3 text-xs text-muted-foreground">
            🔒 安全说明：所有密码与 Token 入库前均使用系统 `APP_SECRET` 进行高强度 AES-GCM 加密，API 和前端绝不暴露明文凭据。
          </div>

          <div className="flex items-center justify-end gap-3 pt-2">
            <button
              type="button"
              onClick={onClose}
              className="rounded-xl border px-4 py-2 text-sm font-medium hover:bg-secondary"
            >
              取消
            </button>
            <button
              type="submit"
              disabled={submitting}
              className="flex items-center gap-2 rounded-xl bg-primary px-5 py-2 text-sm font-medium text-primary-foreground shadow hover:bg-primary/90 disabled:opacity-50"
            >
              {submitting ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
              {isEdit ? "保存修改" : "确认添加"}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
