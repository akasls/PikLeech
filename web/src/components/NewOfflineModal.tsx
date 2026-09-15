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

      const res = await api.submitOffline(lines.length === 1 ? lines[0] : lines)
      setResult(res)
      onSuccess()
    } catch (err: any) {
      setError(err.message || "创建离线任务失败")
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4 animate-in fade-in duration-200">
      <div className="flex flex-col w-full max-w-lg rounded-2xl border bg-card p-6 shadow-2xl animate-in zoom-in-95">
        <div className="flex items-center justify-between border-b pb-3 mb-4">
          <div className="flex items-center gap-2 text-foreground font-semibold text-base">
            <DownloadCloud className="h-5 w-5 text-primary" />
            新建离线下载
          </div>
          <button
            onClick={onClose}
            className="flex h-8 w-8 items-center justify-center rounded-xl text-muted-foreground hover:text-foreground hover:bg-secondary transition-colors"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        {error && (
          <div className="mb-4 flex items-center gap-2 rounded-xl bg-destructive/15 p-3 text-sm text-destructive">
            <AlertCircle className="h-4 w-4 shrink-0" />
            <span>{error}</span>
          </div>
        )}

        {result && (
          <div className="mb-4 rounded-xl border bg-secondary/50 p-3 text-sm space-y-2">
            <div className="flex items-center gap-2 text-emerald-600 dark:text-emerald-400 font-medium">
              <CheckCircle2 className="h-4 w-4" />
              离线任务已成功下发！
            </div>
            {result.results && (
              <div className="max-h-32 overflow-y-auto space-y-1 pt-1">
                {result.results.map((r: any, idx: number) => (
                  <div key={idx} className="flex items-center justify-between text-xs py-1 border-b border-border/50">
                    <span className="truncate max-w-[240px] font-mono">{r.url}</span>
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
            <label className="text-xs font-medium text-muted-foreground">
              下载链接 (支持 Magnet 磁力、HTTP、HTTPS、ED2K 等，多条每行一个)
            </label>
            <textarea
              required
              autoFocus
              rows={6}
              placeholder={"magnet:?xt=urn:btih:...\nhttps://example.com/movie.mp4"}
              value={urlsText}
              onChange={(e) => setUrlsText(e.target.value)}
              className="mt-2 w-full rounded-xl border bg-background p-3 text-sm font-mono placeholder:text-muted-foreground/50 focus:outline-none focus:ring-1 focus:ring-primary leading-relaxed"
            />
          </div>

          <div className="flex items-center justify-end gap-2.5 pt-2">
            <button
              type="button"
              onClick={onClose}
              className="rounded-xl border px-4 py-2 text-xs sm:text-sm font-medium hover:bg-secondary transition-colors"
            >
              关闭
            </button>
            <button
              type="submit"
              disabled={submitting || !urlsText.trim()}
              className="flex items-center gap-2 rounded-xl bg-primary px-5 py-2 text-xs sm:text-sm font-medium text-primary-foreground shadow hover:bg-primary/90 disabled:opacity-50 transition-colors"
            >
              {submitting ? (
                <>
                  <Loader2 className="h-4 w-4 animate-spin" />
                  正在下发...
                </>
              ) : (
                "立即提交"
              )}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
