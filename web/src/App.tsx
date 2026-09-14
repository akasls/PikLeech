import React, { useState, useEffect } from "react"
import { Navbar } from "./components/Navbar"
import { Sidebar, PageTab } from "./components/Sidebar"
import { NewOfflineModal } from "./components/NewOfflineModal"
import { DashboardPage } from "./pages/DashboardPage"
import { FilesPage } from "./pages/FilesPage"
import { TasksPage } from "./pages/TasksPage"
import { AccountsPage } from "./pages/AccountsPage"
import { ApiKeysPage } from "./pages/ApiKeysPage"
import { AuditPage } from "./pages/AuditPage"
import { LoginPage } from "./pages/LoginPage"
import { api } from "./lib/api"
import { Loader2 } from "lucide-react"

export const App: React.FC = () => {
  const [currentUser, setCurrentUser] = useState<string | null>(null)
  const [authChecking, setAuthChecking] = useState(true)
  const [currentTab, setCurrentTab] = useState<PageTab>("dashboard")
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false)
  const [showNewOffline, setShowNewOffline] = useState(false)
  const [isDarkMode, setIsDarkMode] = useState(() => {
    return localStorage.getItem("pikpak_theme") === "dark" ||
      (!localStorage.getItem("pikpak_theme") && window.matchMedia("(prefers-color-scheme: dark)").matches)
  })

  useEffect(() => {
    if (isDarkMode) {
      document.documentElement.classList.add("dark")
      localStorage.setItem("pikpak_theme", "dark")
    } else {
      document.documentElement.classList.remove("dark")
      localStorage.setItem("pikpak_theme", "light")
    }
  }, [isDarkMode])

  useEffect(() => {
    api
      .getMe()
      .then((res) => {
        if (res && res.username) {
          setCurrentUser(res.username)
        }
      })
      .catch(() => {
        setCurrentUser(null)
      })
      .finally(() => {
        setAuthChecking(false)
      })
  }, [])

  const handleLogout = async () => {
    try {
      await api.logout()
    } finally {
      setCurrentUser(null)
    }
  }

  if (authChecking) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background">
        <div className="flex flex-col items-center gap-3 text-muted-foreground">
          <Loader2 className="h-8 w-8 animate-spin text-primary" />
          <span className="text-sm font-medium">系统加载中...</span>
        </div>
      </div>
    )
  }

  if (!currentUser) {
    return <LoginPage onLoginSuccess={(u) => setCurrentUser(u)} />
  }

  return (
    <div className="min-h-screen flex flex-col bg-background text-foreground">
      <Navbar
        username={currentUser}
        onLogout={handleLogout}
        onToggleMobileMenu={() => setIsMobileMenuOpen(!isMobileMenuOpen)}
        isDarkMode={isDarkMode}
        onToggleTheme={() => setIsDarkMode(!isDarkMode)}
      />

      <div className="flex flex-1">
        <Sidebar
          currentTab={currentTab}
          onSelectTab={(tab) => setCurrentTab(tab)}
          isMobileOpen={isMobileMenuOpen}
          onCloseMobile={() => setIsMobileMenuOpen(false)}
        />

        <main className="flex-1 p-4 md:p-6 lg:p-8 max-w-7xl mx-auto w-full overflow-y-auto">
          {currentTab === "dashboard" && (
            <DashboardPage
              onOpenNewOffline={() => setShowNewOffline(true)}
              onNavigate={(tab) => setCurrentTab(tab)}
            />
          )}
          {currentTab === "files" && <FilesPage />}
          {currentTab === "tasks" && (
            <TasksPage onOpenNewOffline={() => setShowNewOffline(true)} />
          )}
          {currentTab === "accounts" && <AccountsPage />}
          {currentTab === "apikeys" && <ApiKeysPage />}
          {currentTab === "audit" && <AuditPage />}
        </main>
      </div>

      {showNewOffline && (
        <NewOfflineModal
          onClose={() => setShowNewOffline(false)}
          onSuccess={() => {
            // Can trigger refresh if on tasks page
          }}
        />
      )}
    </div>
  )
}
