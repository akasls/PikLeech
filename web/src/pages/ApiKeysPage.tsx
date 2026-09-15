import React, { useState, useEffect } from "react"
import { KeyRound, Plus, Trash2, Copy, Check, RefreshCw, Loader2, X } from "lucide-react"
import { api, APIKey } from "../lib/api"
import { formatDate } from "../lib/utils"

export const PERMISSION_LABELS: Record<string, string> = {
  "offline:create": "提交离线",
  "offline:read": "查询离线",
  "files:read": "浏览文件",
  "files:write": "管理文件",
  "accounts:read": "账号状态",
  "*": "全部权限",
}

export const AVAILABLE_PERMISSIONS = [
  { id: "offline:create", label: "提交离线任务", desc: "允许调用接口添加磁力/下载链接进行离线下载" },
  { id: "offline:read", label: "查询离线任务", desc: "允许查询离线任务的进度与完成状态" },
  { id: "files:read", label: "浏览网盘文件", desc: "允许读取多账号聚合网盘的文件列表与目录" },
  { id: "files:write", label: "管理网盘文件", desc: "允许重命名与删除网盘文件" },
  { id: "accounts:read", label: "查看账号状态", desc: "允许读取各 PikPak 账号的容量与运行状态" },
  { id: "*", label: "超级权限 (完全控制)", desc: "授予系统所有开放 API 的全部访问与控制权限" },
]

export const ApiKeysPage: React.FC = () => {
  const [keys, setKeys] = useState<APIKey[]>([])
  const [loading, setLoading] = useState(true)
  const [showCreateModal, setShowCreateModal] = useState(false)
  const [name, setName] = useState("")
  const [selectedPerms, setSelectedPerms] = useState<Set<string>>(new Set(["offline:create", "offline:read"]))
  const [createdKey, setCreatedKey] = useState<APIKey | null>(null)
  const [copied, setCopied] = useState(false)
  const [submitting, setSubmitting] = useState(false)

  const loadKeys = async () => {
    setLoading(true)
    try {
      const data = await api.listApiKeys()
      setKeys(data || [])
    } catch (err) {
      console.error(err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadKeys()
  }, [])

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    if (selectedPerms.size === 0) {
      alert("请至少选择一项权限")
      return
    }
    setSubmitting(true)
    try {
      const permsStr = Array.from(selectedPerms).join(",")
      const res = await api.createApiKey(name, permsStr)
      setCreatedKey(res)
      loadKeys()
    } catch (err: any) {
      alert("创建 API Key 失败: " + err.message)
    } finally {
      setSubmitting(false)
    }
  }

  const handleDelete = async (id: number, keyName: string) => {
    if (!confirm(`确定要永久注销 API Key [${keyName}] 吗？`)) return
    try {
      await api.deleteApiKey(id)
      loadKeys()
    } catch (err: any) {
      alert("删除失败: " + err.message)
    }
  }

  const handleToggle = async (id: number, current: boolean) => {
    try {
      await api.toggleApiKey(id, !current)
      loadKeys()
    } catch (err: any) {
      alert("更新失败: " + err.message)
    }
  }

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between border-b pb-3.5">
        <h2 className="text-lg font-bold tracking-tight">API Key 管理</h2>

        <div className="flex items-center gap-2">
          <button
            onClick={loadKeys}
            className="p-2 rounded-xl border hover:bg-secondary text-muted-foreground hover:text-foreground"
            title="刷新"
          >
            <RefreshCw className={`h-4 w-4 ${loading ? "animate-spin" : ""}`} />
          </button>

          <button
            onClick={() => {
              setName("")
              setCreatedKey(null)
              setSelectedPerms(new Set(["offline:create", "offline:read"]))
              setShowCreateModal(true)
            }}
            className="flex items-center gap-1.5 rounded-xl bg-primary px-3.5 py-2 text-xs font-medium text-primary-foreground shadow hover:bg-primary/90 transition-all"
          >
            <Plus className="h-4 w-4" />
            生成新 API Key
          </button>
        </div>
      </div>

      {/* Keys Table */}
      {loading && keys.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 text-muted-foreground gap-3">
          <Loader2 className="h-8 w-8 animate-spin text-primary" />
          <span className="text-sm">正在加载 API Key 列表...</span>
        </div>
      ) : keys.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 text-muted-foreground">
          <KeyRound className="h-16 w-16 stroke-1 text-muted-foreground/40 mb-3" />
          <p className="text-base font-semibold text-foreground">暂无 API Key</p>
        </div>
      ) : (
        <div className="rounded-2xl border bg-card overflow-x-auto shadow-sm">
          <table className="w-full text-left text-sm min-w-[650px]">
            <thead className="border-b bg-secondary/40 text-xs font-semibold text-muted-foreground">
              <tr>
                <th className="px-4 py-3">名称</th>
                <th className="px-4 py-3">Key 前缀</th>
                <th className="px-4 py-3">权限</th>
                <th className="px-4 py-3">状态</th>
                <th className="px-4 py-3">最后调用时间</th>
                <th className="px-4 py-3">创建时间</th>
                <th className="px-4 py-3 text-right">操作</th>
              </tr>
            </thead>
            <tbody className="divide-y">
              {keys.map((k) => (
                <tr key={k.id} className="hover:bg-secondary/20 transition-colors">
                  <td className="px-4 py-3 font-medium">{k.name}</td>
                  <td className="px-4 py-3 font-mono text-xs text-muted-foreground">{k.key_prefix}****</td>
                  <td className="px-4 py-3 text-xs">
                    <div className="flex flex-wrap gap-1">
                      {k.permissions.split(",").map((p) => {
                        const permLabel = PERMISSION_LABELS[p.trim()] || p.trim()
                        return (
                          <span
                            key={p}
                            className="rounded bg-secondary/80 px-2 py-0.5 text-[11px] font-medium text-foreground"
                          >
                            {permLabel}
                          </span>
                        )
                      })}
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <span
                      className={`inline-block rounded-full px-2.5 py-0.5 text-xs font-medium ${
                        k.is_enabled
                          ? "bg-emerald-500/15 text-emerald-600 dark:text-emerald-400"
                          : "bg-destructive/15 text-destructive"
                      }`}
                    >
                      {k.is_enabled ? "启用" : "已禁用"}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-xs text-muted-foreground">{formatDate(k.last_used_at)}</td>
                  <td className="px-4 py-3 text-xs text-muted-foreground">{formatDate(k.created_at)}</td>
                  <td className="px-4 py-3 text-right">
                    <div className="flex items-center justify-end gap-1.5">
                      <button
                        onClick={() => handleToggle(k.id, k.is_enabled)}
                        className="px-2.5 py-1 rounded-lg text-xs font-medium bg-secondary hover:bg-secondary/80"
                      >
                        {k.is_enabled ? "禁用" : "启用"}
                      </button>
                      <button
                        onClick={() => handleDelete(k.id, k.name)}
                        className="p-1.5 rounded-lg text-muted-foreground hover:text-destructive hover:bg-destructive/10"
                        title="删除"
                      >
                        <Trash2 className="h-4 w-4" />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Create Key Modal */}
      {showCreateModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4">
          <div className="w-full max-w-lg rounded-2xl border bg-card p-6 shadow-2xl animate-in zoom-in-95 max-h-[92vh] overflow-y-auto no-scrollbar">
            <div className="flex items-center justify-between border-b pb-3 mb-4">
              <h3 className="font-semibold text-base flex items-center gap-2">
                <KeyRound className="h-4 w-4 text-primary" />
                生成新 API Key
              </h3>
              <button
                onClick={() => {
                  setShowCreateModal(false)
                  setCreatedKey(null)
                }}
                className="p-1 rounded-lg hover:bg-secondary text-muted-foreground"
              >
                <X className="h-5 w-5" />
              </button>
            </div>

            {createdKey ? (
              <div className="space-y-4">
                <div className="rounded-xl bg-emerald-500/10 border border-emerald-500/20 p-4 text-sm text-emerald-600 dark:text-emerald-400">
                  <p className="font-semibold mb-1">API Key 生成成功！</p>
                  <p className="text-xs leading-relaxed">
                    请务必立即复制并妥善保存。完整密钥仅在此处展示一次，关闭后将无法再次查看。
                  </p>
                </div>

                <div className="flex items-center gap-2 rounded-xl border bg-secondary/60 p-2.5">
                  <span className="font-mono text-xs flex-1 break-all select-all font-semibold">
                    {createdKey.full_key}
                  </span>
                  <button
                    onClick={() => copyToClipboard(createdKey.full_key || "")}
                    className="p-1.5 rounded-lg bg-card shadow-sm hover:bg-secondary"
                    title="复制到剪贴板"
                  >
                    {copied ? <Check className="h-4 w-4 text-emerald-500" /> : <Copy className="h-4 w-4" />}
                  </button>
                </div>

                <div className="flex justify-end pt-2">
                  <button
                    onClick={() => {
                      setShowCreateModal(false)
                      setCreatedKey(null)
                    }}
                    className="rounded-xl bg-primary px-5 py-2 text-sm font-medium text-primary-foreground shadow hover:bg-primary/90"
                  >
                    我已安全保存
                  </button>
                </div>
              </div>
            ) : (
              <form onSubmit={handleCreate} className="space-y-4">
                <div>
                  <label className="text-xs font-semibold text-muted-foreground">Key 备注名称 *</label>
                  <input
                    type="text"
                    required
                    placeholder="例如: 自动化离线脚本、家庭媒体库"
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    className="mt-1.5 w-full rounded-xl border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
                  />
                </div>

                <div>
                  <label className="text-xs font-semibold text-muted-foreground block mb-1.5">
                    接口权限分配 (点击卡片勾选)
                  </label>
                  <div className="space-y-1.5 max-h-60 overflow-y-auto no-scrollbar pr-0.5">
                    {AVAILABLE_PERMISSIONS.map((p) => {
                      const checked = selectedPerms.has(p.id)
                      return (
                        <div
                          key={p.id}
                          onClick={() => {
                            const next = new Set(selectedPerms)
                            if (next.has(p.id)) {
                              next.delete(p.id)
                            } else {
                              next.add(p.id)
                            }
                            setSelectedPerms(next)
                          }}
                          className={`flex items-start gap-3 p-2.5 rounded-xl border cursor-pointer transition-all ${
                            checked
                              ? "border-primary/50 bg-primary/5 text-foreground shadow-xs"
                              : "border-border hover:bg-secondary/40 text-muted-foreground"
                          }`}
                        >
                          <input
                            type="checkbox"
                            checked={checked}
                            onChange={() => {}}
                            className="mt-0.5 rounded border-gray-300 h-4 w-4 accent-primary cursor-pointer"
                          />
                          <div className="text-xs flex-1">
                            <div className="font-semibold text-foreground flex items-center justify-between">
                              <span>{p.label}</span>
                              <span className="font-mono text-[10px] text-muted-foreground bg-secondary px-1.5 py-0.5 rounded">
                                {p.id}
                              </span>
                            </div>
                            <p className="text-[11px] text-muted-foreground mt-0.5 leading-tight">{p.desc}</p>
                          </div>
                        </div>
                      )
                    })}
                  </div>
                </div>

                <div className="flex items-center justify-end gap-3 pt-2">
                  <button
                    type="button"
                    onClick={() => setShowCreateModal(false)}
                    className="rounded-xl border px-4 py-2 text-sm font-medium hover:bg-secondary"
                  >
                    取消
                  </button>
                  <button
                    type="submit"
                    disabled={submitting || !name.trim() || selectedPerms.size === 0}
                    className="flex items-center gap-2 rounded-xl bg-primary px-5 py-2 text-sm font-medium text-primary-foreground shadow hover:bg-primary/90 disabled:opacity-50"
                  >
                    {submitting ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
                    立即生成
                  </button>
                </div>
              </form>
            )}
          </div>
        </div>
      )}
    </div>
  )
}

