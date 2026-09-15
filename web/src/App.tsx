import React, { useState, useEffect } from "react"
import { Navbar } from "./components/Navbar"
import { NewOfflineModal } from "./components/NewOfflineModal"
import { FilesPage } from "./pages/FilesPage"
import { SettingsPage } from "./pages/SettingsPage"
import { LoginPage } from "./pages/LoginPage"
import { api } from "./lib/api"
import { Loader2 } from "lucide-react"

export const App: React.FC = () => {
  const [currentUser, setCurrentUser] = useState<string | null>(null)
  const [currentRole, setCurrentRole] = useState<string>("user")
  const [authChecking, setAuthChecking] = useState(true)
  const [currentPage, setCurrentPage] = useState<"files" | "settings">("files")
  const [searchKeyword, setSearchKeyword] = useState("")
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
          setCurrentRole(res.role || "user")
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
      setCurrentRole("user")
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
    return (
      <LoginPage
        onLoginSuccess={(u, r) => {
          setCurrentUser(u)
          setCurrentRole(r || "user")
        }}
      />
    )
  }

  return (
    <div className="min-h-screen flex flex-col bg-background text-foreground w-full max-w-full overflow-x-hidden">
      <Navbar
        username={currentUser}
        currentPage={currentPage}
        onNavigate={(page) => {
          setCurrentPage(page)
          if (page === "files") {
            setSearchKeyword("")
          }
        }}
        isDarkMode={isDarkMode}
        onToggleTheme={() => setIsDarkMode(!isDarkMode)}
        searchQuery={searchKeyword}
        onSearchChange={setSearchKeyword}
        onClearSearch={() => setSearchKeyword("")}
      />

      <main className="flex-1 p-3 sm:p-5 md:p-6 lg:p-8 max-w-7xl mx-auto w-full min-w-0 max-w-full overflow-x-hidden">
        {currentPage === "files" && (
          <FilesPage
            onOpenNewOffline={() => setShowNewOffline(true)}
            searchKeyword={searchKeyword}
            onClearSearch={() => setSearchKeyword("")}
          />
        )}
        {currentPage === "settings" && (
          <SettingsPage
            currentUsername={currentUser}
            currentUserRole={currentRole}
            onUpdateUsername={(u) => setCurrentUser(u)}
            onOpenNewOffline={() => setShowNewOffline(true)}
            onLogout={handleLogout}
          />
        )}
      </main>

      {showNewOffline && (
        <NewOfflineModal
          onClose={() => setShowNewOffline(false)}
          onSuccess={() => {
            // Task submitted
          }}
        />
      )}
    </div>
  )
}
