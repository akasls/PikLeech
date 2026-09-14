import React, { useState, useEffect } from "react"
import {
  Server,
  DownloadCloud,
  HardDrive,
  CheckCircle2,
  AlertTriangle,
  Clock,
  Plus,
  RefreshCw,
} from "lucide-react"
import { api, DashboardStats } from "../lib/api"
import { formatBytes } from "../lib/utils"

interface DashboardPageProps {
  onOpenNewOffline: () => void
  onNavigate: (tab: any) => void
}

export const DashboardPage: React.FC<DashboardPageProps> = ({
  onOpenNewOffline,
  onNavigate,
}) => {
  const [stats, setStats] = useState<DashboardStats | null>(null)
  const [loading, setLoading] = useState(true)

  const loadStats = async () => {
    setLoading(true)
    try {
      const data = await api.getDashboardStats()
      setStats(data)
    } catch (err) {
      console.error(err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadStats()
  }, [])

  const storagePercent =
    stats && stats.total_space > 0
      ? Math.min(100, Math.round((stats.used_space / stats.total_space) * 100))
      : 0

  return (
    <div className="space-y-6">
      {/* Top Banner */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 rounded-2xl border bg-gradient-to-r from-primary/10 via-card to-card p-6 shadow-sm">
        <div>
          <h2 className="text-2xl font-bold tracking-tight">PikPak 多账号聚合网盘</h2>
          <p className="text-sm text-muted-foreground mt-1">
            多账号离线配额自动调度池 · 统一虚拟文件系统 · 毫秒级额度故障转移
          </p>
        </div>
        <div className="flex items-center gap-2.5">
          <button
            onClick={loadStats}
            disabled={loading}
            className="flex items-center gap-1.5 rounded-xl border bg-card px-3.5 py-2 text-sm font-medium hover:bg-secondary transition-colors"
          >
            <RefreshCw className={`h-4 w-4 ${loading ? "animate-spin" : ""}`} />
            刷新
          </button>
          <button
            onClick={onOpenNewOffline}
            className="flex items-center gap-2 rounded-xl bg-primary px-4 py-2 text-sm font-medium text-primary-foreground shadow-sm hover:bg-primary/90 transition-all"
          >
            <Plus className="h-4 w-4" />
            新建离线下载
          </button>
        </div>
      </div>

      {/* Metric Cards Grid */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        {/* Account Pool */}
        <div
          onClick={() => onNavigate("accounts")}
          className="cursor-pointer rounded-2xl border bg-card p-5 shadow-sm hover:border-primary/50 transition-all"
        >
          <div className="flex items-center justify-between">
            <span className="text-sm font-medium text-muted-foreground">PikPak 账号池</span>
            <div className="rounded-xl bg-primary/10 p-2 text-primary">
              <Server className="h-5 w-5" />
            </div>
          </div>
          <div className="mt-3 flex items-baseline gap-2">
            <span className="text-3xl font-bold tracking-tight">{stats?.total_accounts ?? 0}</span>
            <span className="text-xs text-muted-foreground">个账号</span>
          </div>
          <div className="mt-3 flex items-center gap-2 text-xs">
            <span className="flex items-center gap-1 text-emerald-500 font-medium">
              <CheckCircle2 className="h-3.5 w-3.5" />
              {stats?.healthy_accounts ?? 0} 正常
            </span>
            {(stats?.quota_exhausted_count ?? 0) > 0 && (
              <span className="flex items-center gap-1 text-amber-500 font-medium">
                <AlertTriangle className="h-3.5 w-3.5" />
                {stats?.quota_exhausted_count} 额度已耗尽
              </span>
            )}
            {(stats?.cooldown_count ?? 0) > 0 && (
              <span className="flex items-center gap-1 text-blue-500 font-medium">
                <Clock className="h-3.5 w-3.5" />
                {stats?.cooldown_count} 冷却中
              </span>
            )}
          </div>
        </div>

        {/* Offline Tasks */}
        <div
          onClick={() => onNavigate("tasks")}
          className="cursor-pointer rounded-2xl border bg-card p-5 shadow-sm hover:border-primary/50 transition-all"
        >
          <div className="flex items-center justify-between">
            <span className="text-sm font-medium text-muted-foreground">正在运行任务</span>
            <div className="rounded-xl bg-blue-500/10 p-2 text-blue-500">
              <DownloadCloud className="h-5 w-5" />
            </div>
          </div>
          <div className="mt-3 flex items-baseline gap-2">
            <span className="text-3xl font-bold tracking-tight">{stats?.running_tasks ?? 0}</span>
            <span className="text-xs text-muted-foreground">/ {stats?.total_tasks ?? 0} 总任务</span>
          </div>
          <div className="mt-3 text-xs text-muted-foreground">
            已完成 {stats?.completed_tasks ?? 0} 个 · 失败 {stats?.failed_tasks ?? 0} 个
          </div>
        </div>

        {/* Aggregate Storage */}
        <div
          onClick={() => onNavigate("files")}
          className="cursor-pointer rounded-2xl border bg-card p-5 shadow-sm hover:border-primary/50 transition-all"
        >
          <div className="flex items-center justify-between">
            <span className="text-sm font-medium text-muted-foreground">聚合总容量</span>
            <div className="rounded-xl bg-purple-500/10 p-2 text-purple-500">
              <HardDrive className="h-5 w-5" />
            </div>
          </div>
          <div className="mt-3 flex items-baseline gap-2">
            <span className="text-2xl font-bold tracking-tight">
              {formatBytes(stats?.used_space ?? 0)}
            </span>
            <span className="text-xs text-muted-foreground">
              / {formatBytes(stats?.total_space ?? 0)}
            </span>
          </div>
          <div className="mt-3 w-full bg-secondary rounded-full h-1.5 overflow-hidden">
            <div
              className="bg-primary h-full transition-all duration-500"
              style={{ width: `${storagePercent}%` }}
            />
          </div>
        </div>

        {/* Total Cached Files */}
        <div
          onClick={() => onNavigate("files")}
          className="cursor-pointer rounded-2xl border bg-card p-5 shadow-sm hover:border-primary/50 transition-all"
        >
          <div className="flex items-center justify-between">
            <span className="text-sm font-medium text-muted-foreground">网盘文件数</span>
            <div className="rounded-xl bg-emerald-500/10 p-2 text-emerald-500">
              <CheckCircle2 className="h-5 w-5" />
            </div>
          </div>
          <div className="mt-3 flex items-baseline gap-2">
            <span className="text-3xl font-bold tracking-tight">{stats?.total_files ?? 0}</span>
            <span className="text-xs text-muted-foreground">个跨账号文件</span>
          </div>
          <div className="mt-3 text-xs text-muted-foreground">统一虚拟文件层实时索引</div>
        </div>
      </div>

      {/* Feature Highlights Card */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div className="rounded-2xl border bg-card p-5">
          <h4 className="font-semibold text-sm text-foreground mb-1.5">⚡ 额度自动无缝切换</h4>
          <p className="text-xs text-muted-foreground leading-relaxed">
            当某个 PikPak 账号出现 <code className="text-primary font-mono">task_daily_create_limit</code> 时，系统立即锁定该账号并自动选用下一个可用账号继续创建，用户完全无感。
          </p>
        </div>
        <div className="rounded-2xl border bg-card p-5">
          <h4 className="font-semibold text-sm text-foreground mb-1.5">🛡️ 独立代理与安全隔离</h4>
          <p className="text-xs text-muted-foreground leading-relaxed">
            每个账号独立实例化 HTTP Transport，原生支持 HTTP/HTTPS/SOCKS5 代理。所有凭据与 Token 均通过 AES-GCM 高强度加密入库，日志自动脱敏。
          </p>
        </div>
        <div className="rounded-2xl border bg-card p-5">
          <h4 className="font-semibold text-sm text-foreground mb-1.5">🎬 流畅视频 Range 播放</h4>
          <p className="text-xs text-muted-foreground leading-relaxed">
            内置现代化在线播放器，后端流媒体中转支持标准 HTTP Range 206 Partial Content，毫秒级拖拽，同时自动穿透对应账号的专用代理。
          </p>
        </div>
      </div>
    </div>
  )
}
