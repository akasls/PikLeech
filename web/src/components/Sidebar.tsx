import React from "react"
import {
  LayoutDashboard,
  FolderOpen,
  DownloadCloud,
  Server,
  KeyRound,
  FileText,
  X,
} from "lucide-react"

export type PageTab = "dashboard" | "files" | "tasks" | "accounts" | "apikeys" | "audit"

interface SidebarProps {
  currentTab: PageTab
  onSelectTab: (tab: PageTab) => void
  isMobileOpen: boolean
  onCloseMobile: () => void
}

export const Sidebar: React.FC<SidebarProps> = ({
  currentTab,
  onSelectTab,
  isMobileOpen,
  onCloseMobile,
}) => {
  const navItems = [
    { id: "dashboard", label: "系统概览", icon: LayoutDashboard },
    { id: "files", label: "统一网盘", icon: FolderOpen },
    { id: "tasks", label: "离线下载", icon: DownloadCloud },
    { id: "accounts", label: "PikPak 账号", icon: Server },
    { id: "apikeys", label: "API 密钥", icon: KeyRound },
    { id: "audit", label: "审计日志", icon: FileText },
  ]

  const content = (
    <aside className="flex h-full w-64 flex-col border-r bg-card p-4">
      <div className="flex items-center justify-between pb-4 md:hidden">
        <span className="font-semibold text-sm text-muted-foreground">导航菜单</span>
        <button onClick={onCloseMobile} className="p-1 rounded-lg hover:bg-secondary">
          <X className="h-5 w-5" />
        </button>
      </div>

      <nav className="space-y-1.5 flex-1">
        {navItems.map((item) => {
          const Icon = item.icon
          const isActive = currentTab === item.id
          return (
            <button
              key={item.id}
              onClick={() => {
                onSelectTab(item.id as PageTab)
                onCloseMobile()
              }}
              className={`flex w-full items-center gap-3 rounded-xl px-3.5 py-2.5 text-sm font-medium transition-all ${
                isActive
                  ? "bg-primary text-primary-foreground shadow-sm"
                  : "text-muted-foreground hover:bg-secondary hover:text-foreground"
              }`}
            >
              <Icon className={`h-4 w-4 ${isActive ? "text-primary-foreground" : "text-muted-foreground"}`} />
              {item.label}
            </button>
          )
        })}
      </nav>

      <div className="pt-4 border-t text-xs text-muted-foreground text-center">
        PikPak Multi-Account v1.0
      </div>
    </aside>
  )

  return (
    <>
      {/* Desktop Sidebar */}
      <div className="hidden md:block h-[calc(100vh-4rem)] sticky top-16">{content}</div>

      {/* Mobile Drawer */}
      {isMobileOpen && (
        <div className="fixed inset-0 z-40 md:hidden">
          <div className="fixed inset-0 bg-black/50 backdrop-blur-sm" onClick={onCloseMobile} />
          <div className="fixed inset-y-0 left-0 z-50 w-64 shadow-2xl animate-in slide-in-from-left">
            {content}
          </div>
        </div>
      )}
    </>
  )
}
