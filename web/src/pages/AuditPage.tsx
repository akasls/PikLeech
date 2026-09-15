import React, { useState, useEffect } from "react"
import { FileText, RefreshCw, Loader2 } from "lucide-react"
import { api, AuditLog } from "../lib/api"
import { formatDate } from "../lib/utils"

export const AuditPage: React.FC = () => {
  const [logs, setLogs] = useState<AuditLog[]>([])
  const [loading, setLoading] = useState(true)

  const loadLogs = async () => {
    setLoading(true)
    try {
      const data = await api.listAuditLogs()
      setLogs(data.logs || [])
    } catch (err) {
      console.error(err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadLogs()
  }, [])

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between border-b pb-3.5">
        <h2 className="text-lg font-bold tracking-tight">系统审计日志</h2>

        <button
          onClick={loadLogs}
          className="p-2 rounded-xl border hover:bg-secondary text-muted-foreground hover:text-foreground"
          title="刷新"
        >
          <RefreshCw className={`h-4 w-4 ${loading ? "animate-spin" : ""}`} />
        </button>
      </div>

      {loading && logs.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 text-muted-foreground gap-3">
          <Loader2 className="h-8 w-8 animate-spin text-primary" />
          <span className="text-sm">正在加载审计日志...</span>
        </div>
      ) : logs.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 text-muted-foreground">
          <FileText className="h-16 w-16 stroke-1 text-muted-foreground/40 mb-3" />
          <p className="text-base font-semibold text-foreground">暂无审计日志</p>
        </div>
      ) : (
        <div className="rounded-2xl border bg-card overflow-x-auto shadow-sm">
          <table className="w-full text-left text-sm min-w-[600px]">
            <thead className="border-b bg-secondary/40 text-xs font-semibold text-muted-foreground">
              <tr>
                <th className="px-4 py-3 w-40">操作动作</th>
                <th className="px-4 py-3 w-48">目标对象</th>
                <th className="px-4 py-3">详细信息</th>
                <th className="px-4 py-3 w-28">操作状态</th>
                <th className="px-4 py-3 w-28">操作员</th>
                <th className="px-4 py-3 w-40">时间</th>
              </tr>
            </thead>
            <tbody className="divide-y">
              {logs.map((log) => (
                <tr key={log.id} className="hover:bg-secondary/20 transition-colors">
                  <td className="px-4 py-3 font-mono text-xs font-semibold text-primary">{log.action}</td>
                  <td className="px-4 py-3 font-medium text-xs truncate max-w-[160px]">{log.target}</td>
                  <td className="px-4 py-3 text-xs text-muted-foreground truncate max-w-sm" title={log.details}>
                    {log.details || "-"}
                  </td>
                  <td className="px-4 py-3">
                    <span
                      className={`inline-block rounded-full px-2 py-0.5 text-xs font-medium ${
                        log.status === "SUCCESS"
                          ? "bg-emerald-500/15 text-emerald-600 dark:text-emerald-400"
                          : "bg-destructive/15 text-destructive"
                      }`}
                    >
                      {log.status}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-xs font-medium">{log.operator}</td>
                  <td className="px-4 py-3 text-xs text-muted-foreground">{formatDate(log.created_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
