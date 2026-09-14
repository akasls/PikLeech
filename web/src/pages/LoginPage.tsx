import React, { useState } from "react"
import { Lock, User, Loader2, AlertCircle } from "lucide-react"
import { api } from "../lib/api"

interface LoginPageProps {
  onLoginSuccess: (username: string) => void
}

export const LoginPage: React.FC<LoginPageProps> = ({ onLoginSuccess }) => {
  const [username, setUsername] = useState("")
  const [password, setPassword] = useState("")
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState("")

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    setError("")

    try {
      const res = await api.login(username.trim(), password)
      if (res.success) {
        onLoginSuccess(res.username || username)
      }
    } catch (err: any) {
      setError(err.message || "用户名或密码错误")
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="flex min-h-screen w-full items-center justify-center bg-background px-4">
      <div className="w-full max-w-md rounded-3xl border bg-card p-8 shadow-2xl space-y-6">
        <div className="text-center space-y-2">
          <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-primary text-primary-foreground font-black text-2xl shadow-lg shadow-primary/25">
            P
          </div>
          <h1 className="text-2xl font-bold tracking-tight">PikPak 聚合网盘</h1>
          <p className="text-xs text-muted-foreground">
            多账号离线额度自动调度 · 统一虚拟云盘管理系统
          </p>
        </div>

        {error && (
          <div className="flex items-center gap-2 rounded-xl bg-destructive/15 p-3 text-xs text-destructive">
            <AlertCircle className="h-4 w-4 shrink-0" />
            <span>{error}</span>
          </div>
        )}

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
              管理员用户名
            </label>
            <div className="relative mt-1.5">
              <User className="absolute left-3.5 top-2.5 h-4 w-4 text-muted-foreground" />
              <input
                type="text"
                required
                placeholder="默认: admin"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                className="w-full rounded-xl border bg-background pl-10 pr-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
              />
            </div>
          </div>

          <div>
            <label className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
              密码
            </label>
            <div className="relative mt-1.5">
              <Lock className="absolute left-3.5 top-2.5 h-4 w-4 text-muted-foreground" />
              <input
                type="password"
                required
                placeholder="管理员密码"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                className="w-full rounded-xl border bg-background pl-10 pr-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
              />
            </div>
          </div>

          <button
            type="submit"
            disabled={loading || !username.trim() || !password}
            className="flex w-full items-center justify-center gap-2 rounded-xl bg-primary py-2.5 text-sm font-semibold text-primary-foreground shadow-lg shadow-primary/20 hover:bg-primary/90 disabled:opacity-50 transition-all"
          >
            {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
            登录管理系统
          </button>
        </form>

        <div className="pt-2 text-center text-xs text-muted-foreground border-t">
          支持 Docker 一键部署 · 本地 SQLite 持久化
        </div>
      </div>
    </div>
  )
}
