import React, { useState, useEffect } from "react"
import {
  DownloadCloud,
  RefreshCw,
  RotateCcw,
  XCircle,
  Trash2,
  Plus,
  CheckCircle2,
  AlertCircle,
  Clock,
  Loader2,
} from "lucide-react"
import { api, OfflineTask } from "../lib/api"
import { formatDate } from "../lib/utils"

interface TasksPageProps {
  onOpenNewOffline: () => void
}

export const TasksPage: React.FC<TasksPageProps> = ({ onOpenNewOffline }) => {
  const [tasks, setTasks] = useState<OfflineTask[]>([])
  const [loading, setLoading] = useState(true)
  const [statusFilter, setStatusFilter] = useState("")

  const loadTasks = async () => {
    setLoading(true)
    try {
      const data = await api.listTasks(statusFilter)
      setTasks(data.tasks || [])
    } catch (err) {
      console.error(err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadTasks()
    const timer = setInterval(() => {
      // Background auto-refresh for running tasks
      api.listTasks(statusFilter).then((d) => setTasks(d.tasks || []))
    }, 4000)
    return () => clearInterval(timer)
  }, [statusFilter])

  const handleCancel = async (id: string) => {
    try {
      await api.cancelTask(id)
      loadTasks()
    } catch (err: any) {
      alert("取消任务失败: " + err.message)
    }
  }

  const handleRetry = async (id: string) => {
    try {
      await api.retryTask(id)
      loadTasks()
    } catch (err: any) {
      alert("重试任务失败: " + err.message)
    }
  }

  const handleDelete = async (id: string) => {
    if (!confirm("确定要删除该离线任务记录吗？")) return
    try {
      await api.deleteTask(id)
      loadTasks()
    } catch (err: any) {
      alert("删除记录失败: " + err.message)
    }
  }

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
            <XCircle className="h-3.5 w-3.5" />
            已取消
          </span>
        )
      default:
        return (
          <span className="inline-flex items-center gap-1 rounded-full bg-secondary px-2.5 py-0.5 text-xs font-medium text-muted-foreground">
            <Clock className="h-3.5 w-3.5" />
            {status}
          </span>
        )
    }
  }

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 border-b pb-4">
        <div>
          <h2 className="text-xl font-bold tracking-tight">离线任务管理</h2>
          <p className="text-xs text-muted-foreground mt-0.5">
            实时查看 PikPak 云端离线下载进度，支持失败重试与多账号自动调度记录
          </p>
        </div>

        <div className="flex items-center gap-2">
          {/* Status Filter */}
          <select
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
            className="h-9 rounded-xl border bg-card px-3 text-xs text-foreground focus:outline-none"
          >
            <option value="">全部状态</option>
            <option value="RUNNING">下载中</option>
            <option value="COMPLETE">已完成</option>
            <option value="ERROR">失败</option>
            <option value="CANCELLED">已取消</option>
          </select>

          <button
            onClick={loadTasks}
            className="p-2 rounded-xl border hover:bg-secondary text-muted-foreground hover:text-foreground"
            title="刷新"
          >
            <RefreshCw className={`h-4 w-4 ${loading ? "animate-spin" : ""}`} />
          </button>

          <button
            onClick={onOpenNewOffline}
            className="flex items-center gap-1.5 rounded-xl bg-primary px-3.5 py-2 text-xs font-medium text-primary-foreground shadow hover:bg-primary/90 transition-all"
          >
            <Plus className="h-4 w-4" />
            新建离线任务
          </button>
        </div>
      </div>

      {/* Task List */}
      {loading && tasks.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 text-muted-foreground gap-3">
          <Loader2 className="h-8 w-8 animate-spin text-primary" />
          <span className="text-sm">正在获取离线任务列表...</span>
        </div>
      ) : tasks.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 text-muted-foreground">
          <DownloadCloud className="h-16 w-16 stroke-1 text-muted-foreground/40 mb-3" />
          <p className="text-base font-semibold text-foreground">暂无离线任务</p>
          <p className="text-xs text-muted-foreground mt-1">点击右上角“新建离线任务”开始下载！</p>
        </div>
      ) : (
        <div className="rounded-2xl border bg-card overflow-hidden shadow-sm">
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm">
              <thead className="border-b bg-secondary/40 text-xs font-semibold text-muted-foreground">
                <tr>
                  <th className="px-4 py-3">任务名称 / 原始链接</th>
                  <th className="px-4 py-3 w-28">状态</th>
                  <th className="px-4 py-3 w-44">下载进度</th>
                  <th className="px-4 py-3 w-32">执行账号</th>
                  <th className="px-4 py-3 w-36">提交时间</th>
                  <th className="px-4 py-3 w-28 text-right">操作</th>
                </tr>
              </thead>
              <tbody className="divide-y">
                {tasks.map((task) => (
                  <tr key={task.id} className="hover:bg-secondary/20 transition-colors">
                    <td className="px-4 py-3 max-w-xs md:max-w-md">
                      <div className="font-medium truncate" title={task.file_name || task.source_url}>
                        {task.file_name || "正在解析资源..."}
                      </div>
                      <div className="text-xs font-mono text-muted-foreground truncate" title={task.source_url}>
                        {task.source_url}
                      </div>
                      {task.error_message && (
                        <div className="text-[11px] text-destructive mt-0.5 truncate" title={task.error_message}>
                          错误: {task.error_message}
                        </div>
                      )}
                    </td>
                    <td className="px-4 py-3">{getStatusBadge(task.status)}</td>
                    <td className="px-4 py-3">
                      <div className="space-y-1">
                        <div className="flex items-center justify-between text-xs text-muted-foreground">
                          <span>{task.progress}%</span>
                          {task.status === "COMPLETE" && <span>完成</span>}
                        </div>
                        <div className="w-full bg-secondary rounded-full h-1.5 overflow-hidden">
                          <div
                            className={`h-full transition-all duration-300 ${
                              task.status === "COMPLETE"
                                ? "bg-emerald-500"
                                : task.status === "ERROR"
                                ? "bg-destructive"
                                : "bg-primary"
                            }`}
                            style={{ width: `${task.progress}%` }}
                          />
                        </div>
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <span className="inline-block rounded-full bg-secondary px-2.5 py-0.5 text-xs text-muted-foreground">
                        {task.account_name}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-xs text-muted-foreground">
                      {formatDate(task.created_at)}
                    </td>
                    <td className="px-4 py-3 text-right">
                      <div className="flex items-center justify-end gap-1.5">
                        {task.status === "ERROR" && (
                          <button
                            onClick={() => handleRetry(task.id)}
                            className="p-1 rounded-lg text-primary hover:bg-primary/10"
                            title="重新调度执行"
                          >
                            <RotateCcw className="h-4 w-4" />
                          </button>
                        )}
                        {(task.status === "RUNNING" || task.status === "PENDING") && (
                          <button
                            onClick={() => handleCancel(task.id)}
                            className="p-1 rounded-lg text-amber-500 hover:bg-amber-500/10"
                            title="取消任务"
                          >
                            <XCircle className="h-4 w-4" />
                          </button>
                        )}
                        <button
                          onClick={() => handleDelete(task.id)}
                          className="p-1 rounded-lg text-muted-foreground hover:text-destructive hover:bg-destructive/10"
                          title="删除记录"
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
        </div>
      )}
    </div>
  )
}
