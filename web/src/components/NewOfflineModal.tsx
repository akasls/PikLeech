import React, { useState } from "react"
import { X, DownloadCloud, CheckCircle2, AlertCircle, Loader2 } from "lucide-react"
import { api } from "../lib/api"

interface NewOfflineModalProps {
  onClose: () => void
  onSuccess: () => void
}

export const NewOfflineModal: React.FC<NewOfflineModalProps> = ({
  onClose,
  onSuccess,
}) => {
  const [urlsText, setUrlsText] = useState("")
  const [name, setName] = useState("")
  const [submitting, setSubmitting] = useState(false)
  const [result, setResult] = useState<any>(null)
  const [error, setError] = useState("")

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!urlsText.trim()) return

    setSubmitting(true)
    setError("")
    setResult(null)

    try {
      const lines = urlsText
        .split("\n")
        .map((l) => l.trim())
        .filter((l) => l.length > 0)

      const res = await api.submitOffline(lines.length === 1 ? lines[0] : lines, name)
      setResult(res)
      onSuccess()
    } catch (err: any) {
      setError(err.message || "创建离线任务失败")
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4">
      <div className="flex flex-col w-full max-w-xl rounded-2xl border bg-card p-6 shadow-2xl animate-in zoom-in-95">
        <div className="flex items-center justify-between border-b pb-3 mb-4">
          <div className="flex items-center gap-2 text-primary font-semibold text-base">
            <DownloadCloud className="h-5 w-5" />
            新建 PikPak 离线下载
          </div>
          <button onClick={onClose} className="p-1 rounded-lg hover:bg-secondary">
            <X className="h-5 w-5" />
          </button>
        </div>

        {error && (
          <div className="mb-4 flex items-center gap-2 rounded-xl bg-destructive/15 p-3 text-sm text-destructive">
            <AlertCircle className="h-4 w-4 shrink-0" />
            <span>{error}</span>
          </div>
        )}

        {result && (
          <div className="mb-4 rounded-xl border bg-secondary/50 p-4 text-sm space-y-2">
            <div className="flex items-center gap-2 text-emerald-600 dark:text-emerald-400 font-medium">
              <CheckCircle2 className="h-4 w-4" />
              离线任务创建成功！
            </div>
            {result.account && (
              <div className="text-xs text-muted-foreground">
                实际调度账号: <span className="font-semibold text-foreground">{result.account}</span>
              </div>
            )}
            {result.results && (
              <div className="max-h-40 overflow-y-auto space-y-1 pt-2">
                {result.results.map((r: any, idx: number) => (
                  <div key={idx} className="flex items-center justify-between text-xs py-1 border-b">
                    <span className="truncate max-w-xs">{r.url}</span>
                    <span className={r.success ? "text-emerald-500 font-medium" : "text-destructive"}>
                      {r.success ? "已下发" : r.error || "失败"}
                    </span>
                  </div>
                ))}
              </div>
            )}
          </div>
        )}

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
              下载链接 (支持 Magnet, HTTP, HTTPS, ED2K，多条每行一个)
            </label>
            <textarea
              required
              rows={5}
              placeholder="magnet:?xt=urn:btih:...\nhttps://example.com/file.zip"
              value={urlsText}
              onChange={(e) => setUrlsText(e.target.value)}
              className="mt-1.5 w-full rounded-xl border bg-background p-3 text-sm font-mono placeholder:text-muted-foreground/60 focus:outline-none focus:ring-2 focus:ring-primary"
            />
          </div>

          <div>
            <label className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
              自定义文件名 (可选，单条有效)
            </label>
            <input
              type="text"
              placeholder="留空自动使用云端文件名"
              value={name}
              onChange={(e) => setName(e.target.value)}
              className="mt-1.5 w-full rounded-xl border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
            />
          </div>

          <div className="rounded-xl bg-secondary/40 p-3 text-xs text-muted-foreground">
            💡 调度提示：系统将自动在所有已启用账号中按“优先级 + 轮询”分配任务。若当前账号今日离线额度耗尽，系统将毫秒级自动切换下一个可用账号继续执行！
          </div>

          <div className="flex items-center justify-end gap-3 pt-2">
            <button
              type="button"
              onClick={onClose}
              className="rounded-xl border px-4 py-2 text-sm font-medium hover:bg-secondary"
            >
              关闭
            </button>
            <button
              type="submit"
              disabled={submitting || !urlsText.trim()}
              className="flex items-center gap-2 rounded-xl bg-primary px-5 py-2 text-sm font-medium text-primary-foreground shadow hover:bg-primary/90 disabled:opacity-50"
            >
              {submitting ? (
                <>
                  <Loader2 className="h-4 w-4 animate-spin" />
                  正在自动调度与创建...
                </>
              ) : (
                "立即提交离线任务"
              )}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
