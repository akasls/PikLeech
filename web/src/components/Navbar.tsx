import React, { useState } from "react"
import { Moon, Sun, User, Key, LogOut, Menu } from "lucide-react"
import { api } from "../lib/api"

interface NavbarProps {
  username: string
  onLogout: () => void
  onToggleMobileMenu: () => void
  isDarkMode: boolean
  onToggleTheme: () => void
}

export const Navbar: React.FC<NavbarProps> = ({
  username,
  onLogout,
  onToggleMobileMenu,
  isDarkMode,
  onToggleTheme,
}) => {
  const [showUserMenu, setShowUserMenu] = useState(false)
  const [showPasswordModal, setShowPasswordModal] = useState(false)
  const [oldPass, setOldPass] = useState("")
  const [newPass, setNewPass] = useState("")
  const [passError, setPassError] = useState("")
  const [passSuccess, setPassSuccess] = useState("")

  const handleChangePassword = async (e: React.FormEvent) => {
    e.preventDefault()
    setPassError("")
    setPassSuccess("")
    try {
      await api.changePassword(oldPass, newPass)
      setPassSuccess("密码修改成功！")
      setOldPass("")
      setNewPass("")
      setTimeout(() => setShowPasswordModal(false), 1500)
    } catch (err: any) {
      setPassError(err.message || "修改密码失败")
    }
  }

  return (
    <>
      <header className="sticky top-0 z-30 flex h-16 w-full items-center justify-between border-b bg-card/80 px-4 md:px-6 backdrop-blur">
        <div className="flex items-center gap-3">
          <button
            onClick={onToggleMobileMenu}
            className="md:hidden p-2 rounded-lg hover:bg-secondary"
            title="菜单"
          >
            <Menu className="h-5 w-5" />
          </button>
          <div className="flex items-center gap-2.5">
            <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-primary text-primary-foreground font-bold shadow-sm">
              P
            </div>
            <div>
              <span className="font-semibold text-lg tracking-tight">PikPak 聚合网盘</span>
              <span className="ml-2 hidden sm:inline-block rounded-full bg-primary/10 px-2 py-0.5 text-xs font-medium text-primary">
                Multi-Account
              </span>
            </div>
          </div>
        </div>

        <div className="flex items-center gap-2">
          <button
            onClick={onToggleTheme}
            className="p-2 rounded-lg text-muted-foreground hover:text-foreground hover:bg-secondary transition-colors"
            title={isDarkMode ? "切换亮色模式" : "切换暗色模式"}
          >
            {isDarkMode ? <Sun className="h-5 w-5" /> : <Moon className="h-5 w-5" />}
          </button>

          <div className="relative">
            <button
              onClick={() => setShowUserMenu(!showUserMenu)}
              className="flex items-center gap-2 rounded-full border bg-secondary/60 py-1.5 px-3 hover:bg-secondary transition-colors"
            >
              <User className="h-4 w-4 text-primary" />
              <span className="text-sm font-medium">{username}</span>
            </button>

            {showUserMenu && (
              <div
                className="absolute right-0 mt-2 w-48 rounded-xl border bg-card p-1.5 shadow-lg z-50 animate-in fade-in zoom-in-95"
                onMouseLeave={() => setShowUserMenu(false)}
              >
                <button
                  onClick={() => {
                    setShowUserMenu(false)
                    setShowPasswordModal(true)
                  }}
                  className="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-sm text-foreground hover:bg-secondary"
                >
                  <Key className="h-4 w-4 text-muted-foreground" />
                  修改密码
                </button>
                <div className="my-1 border-t" />
                <button
                  onClick={() => {
                    setShowUserMenu(false)
                    onLogout()
                  }}
                  className="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-sm text-destructive hover:bg-destructive/10"
                >
                  <LogOut className="h-4 w-4" />
                  退出登录
                </button>
              </div>
            )}
          </div>
        </div>
      </header>

      {/* Change Password Modal */}
      {showPasswordModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4">
          <div className="w-full max-w-md rounded-2xl border bg-card p-6 shadow-2xl">
            <h3 className="text-lg font-semibold mb-4">修改管理员密码</h3>
            {passError && (
              <div className="mb-4 rounded-lg bg-destructive/15 p-3 text-sm text-destructive">
                {passError}
              </div>
            )}
            {passSuccess && (
              <div className="mb-4 rounded-lg bg-emerald-500/15 p-3 text-sm text-emerald-600 dark:text-emerald-400">
                {passSuccess}
              </div>
            )}
            <form onSubmit={handleChangePassword} className="space-y-4">
              <div>
                <label className="text-sm font-medium text-muted-foreground">当前原密码</label>
                <input
                  type="password"
                  required
                  value={oldPass}
                  onChange={(e) => setOldPass(e.target.value)}
                  className="mt-1.5 w-full rounded-lg border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
                />
              </div>
              <div>
                <label className="text-sm font-medium text-muted-foreground">新密码 (至少6位)</label>
                <input
                  type="password"
                  required
                  value={newPass}
                  onChange={(e) => setNewPass(e.target.value)}
                  className="mt-1.5 w-full rounded-lg border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
                />
              </div>
              <div className="mt-6 flex justify-end gap-2.5">
                <button
                  type="button"
                  onClick={() => setShowPasswordModal(false)}
                  className="rounded-lg border px-4 py-2 text-sm font-medium hover:bg-secondary"
                >
                  取消
                </button>
                <button
                  type="submit"
                  className="rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground shadow hover:bg-primary/90"
                >
                  确认修改
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </>
  )
}
