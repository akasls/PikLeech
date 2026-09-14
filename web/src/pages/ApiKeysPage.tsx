import React, { useState, useEffect } from "react"
import { KeyRound, Plus, Trash2, Copy, Check, RefreshCw, Loader2, X, Terminal } from "lucide-react"
import { api, APIKey } from "../lib/api"
import { formatDate } from "../lib/utils"

export const ApiKeysPage: React.FC = () => {
  const [keys, setKeys] = useState<APIKey[]>([])
  const [loading, setLoading] = useState(true)
  const [showCreateModal, setShowCreateModal] = useState(false)
  const [name, setName] = useState("")
  const [permissions, setPermissions] = useState("offline:create,offline:read")
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
    setSubmitting(true)
    try {
      const res = await api.createApiKey(name, permissions)
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
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 border-b pb-4">
        <div>
          <h2 className="text-xl font-bold tracking-tight">外部 REST API Key 管理</h2>
          <p className="text-xs text-muted-foreground mt-0.5">
            用于通过外部脚本、下载器、自动化工作流等程序直接调用本系统离线下载与查询接口
          </p>
        </div>

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
          <p className="text-xs text-muted-foreground mt-1">生成一个 API Key 以便自动化程序调用离线下载接口</p>
        </div>
      ) : (
        <div className="rounded-2xl border bg-card overflow-hidden shadow-sm">
          <table className="w-full text-left text-sm">
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
                    <span className="rounded bg-secondary px-2 py-0.5 font-mono">{k.permissions}</span>
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

      {/* Developer API Documentation Banner */}
      <div className="rounded-2xl border bg-card p-6 shadow-sm space-y-4">
        <div className="flex items-center gap-2 text-primary font-semibold text-sm">
          <Terminal className="h-5 w-5" />
          外部 REST API 调用指引与 curl 示例
        </div>

        <div className="space-y-3 text-xs font-mono">
          <div>
            <p className="text-muted-foreground font-sans font-medium mb-1">
              1. 提交离线下载 (支持 Idempotency-Key 幂等防重):
            </p>
            <pre className="rounded-xl bg-secondary/80 p-3 overflow-x-auto text-foreground">
{`curl -X POST http://localhost:8080/api/v1/offline \\
  -H "Authorization: Bearer <YOUR_API_KEY>" \\
  -H "Content-Type: application/json" \\
  -H "Idempotency-Key: custom-uuid-123456" \\
  -d '{"url": "magnet:?xt=urn:btih:..."}'`}
            </pre>
          </div>

          <div>
            <p className="text-muted-foreground font-sans font-medium mb-1">
              2. 查询任务状态:
            </p>
            <pre className="rounded-xl bg-secondary/80 p-3 overflow-x-auto text-foreground">
{`curl -X GET http://localhost:8080/api/v1/offline/<TASK_ID> \\
  -H "Authorization: Bearer <YOUR_API_KEY>"`}
            </pre>
          </div>
        </div>
      </div>

      {/* Create Key Modal */}
      {showCreateModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4">
          <div className="w-full max-w-md rounded-2xl border bg-card p-6 shadow-2xl animate-in zoom-in-95">
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
                className="p-1 rounded-lg hover:bg-secondary"
              >
                <X className="h-5 w-5" />
              </button>
            </div>

            {createdKey ? (
              <div className="space-y-4">
                <div className="rounded-xl bg-emerald-500/10 border border-emerald-500/20 p-4 text-sm text-emerald-600 dark:text-emerald-400">
                  <p className="font-semibold mb-1">API Key 生成成功！</p>
                  <p className="text-xs leading-relaxed">
                    请务必立即复制并安全保存。出于安全考虑，完整密钥仅在此处展示一次，数据库只保存 SHA-256 摘要。
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
                  <label className="text-xs font-semibold text-muted-foreground uppercase">Key 备注名称 *</label>
                  <input
                    type="text"
                    required
                    placeholder="例如: HomeServer-Automation"
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    className="mt-1.5 w-full rounded-xl border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
                  />
                </div>

                <div>
                  <label className="text-xs font-semibold text-muted-foreground uppercase">权限分配</label>
                  <input
                    type="text"
                    required
                    value={permissions}
                    onChange={(e) => setPermissions(e.target.value)}
                    className="mt-1.5 w-full rounded-xl border bg-background px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-primary"
                  />
                  <span className="text-[11px] text-muted-foreground mt-1 block">
                    默认: offline:create,offline:read (支持通配符 *)
                  </span>
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
                    disabled={submitting || !name.trim()}
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
