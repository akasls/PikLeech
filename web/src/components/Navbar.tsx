import React, { useState } from "react"
import { Moon, Sun, Settings, Folder, Search, X } from "lucide-react"

interface NavbarProps {
  username: string
  currentPage: "files" | "settings"
  onNavigate: (page: "files" | "settings") => void
  isDarkMode: boolean
  onToggleTheme: () => void
  searchQuery?: string
  onSearchChange?: (query: string) => void
  onSearchSubmit?: () => void
  onClearSearch?: () => void
}

export const Navbar: React.FC<NavbarProps> = ({
  currentPage,
  onNavigate,
  isDarkMode,
  onToggleTheme,
  searchQuery = "",
  onSearchChange,
  onSearchSubmit,
  onClearSearch,
}) => {
  const [mobileSearchOpen, setMobileSearchOpen] = useState(false)

  const handleFormSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    onSearchSubmit?.()
  }

  const handleCloseMobileSearch = () => {
    setMobileSearchOpen(false)
    if (searchQuery) {
      onClearSearch?.()
    }
  }

  return (
    <header className="sticky top-0 z-30 flex h-14 sm:h-16 w-full items-center justify-between border-b bg-card/80 px-3 sm:px-6 md:px-8 backdrop-blur shadow-sm">
      {/* Brand / Logo */}
      <div className={`items-center gap-2.5 ${mobileSearchOpen && currentPage === "files" ? "hidden sm:flex" : "flex"}`}>
        <div
          onClick={() => onNavigate("files")}
          className="flex items-center gap-2.5 cursor-pointer select-none group"
        >
          <img
            src="/logo.png"
            alt="PikLeech"
            className="w-7 h-7 rounded-lg shadow-sm object-cover group-hover:scale-105 transition-transform"
          />
          <span className="font-extrabold text-base sm:text-lg tracking-tight hover:opacity-90 transition-opacity">
            PikLeech
          </span>
        </div>
      </div>

      {/* Right side controls */}
      <div className="flex items-center gap-1.5 sm:gap-2 flex-1 sm:flex-initial justify-end">
        {currentPage === "files" && (
          <>
            {/* Desktop Search Component */}
            <form onSubmit={handleFormSubmit} className="relative hidden md:flex items-center">
              <Search className="absolute left-2.5 h-3.5 w-3.5 text-muted-foreground pointer-events-none" />
              <input
                type="text"
                placeholder="搜索聚合网盘文件..."
                value={searchQuery}
                onChange={(e) => onSearchChange?.(e.target.value)}
                className="h-9 w-48 lg:w-64 rounded-xl border bg-background/90 pl-8 pr-7 text-xs focus:outline-none focus:ring-1 focus:ring-primary transition-all placeholder:text-muted-foreground/70"
              />
              {searchQuery && (
                <button
                  type="button"
                  onClick={onClearSearch}
                  className="absolute right-2 p-0.5 rounded text-muted-foreground hover:text-foreground"
                >
                  <X className="h-3.5 w-3.5" />
                </button>
              )}
            </form>

            {/* Mobile Search Component */}
            <div className="flex md:hidden items-center flex-1 justify-end">
              {mobileSearchOpen ? (
                <form onSubmit={handleFormSubmit} className="relative flex items-center w-full max-w-xs animate-in fade-in zoom-in-95">
                  <Search className="absolute left-2.5 h-3.5 w-3.5 text-muted-foreground pointer-events-none" />
                  <input
                    autoFocus
                    type="text"
                    placeholder="搜索文件..."
                    value={searchQuery}
                    onChange={(e) => onSearchChange?.(e.target.value)}
                    className="h-9 w-full rounded-xl border bg-background pl-8 pr-7 text-xs focus:outline-none focus:ring-1 focus:ring-primary placeholder:text-muted-foreground/70"
                  />
                  <button
                    type="button"
                    onClick={handleCloseMobileSearch}
                    className="absolute right-2 p-1 rounded-lg text-muted-foreground hover:text-foreground"
                  >
                    <X className="h-3.5 w-3.5" />
                  </button>
                </form>
              ) : (
                <button
                  onClick={() => setMobileSearchOpen(true)}
                  className="flex h-9 w-9 items-center justify-center rounded-xl text-muted-foreground hover:text-foreground hover:bg-secondary transition-colors"
                  title="搜索文件"
                >
                  <Search className="h-4 w-4" />
                </button>
              )}
            </div>
          </>
        )}

        {/* Theme toggle */}
        <button
          onClick={onToggleTheme}
          className="flex h-9 w-9 items-center justify-center rounded-xl text-muted-foreground hover:text-foreground hover:bg-secondary transition-colors shrink-0"
          title={isDarkMode ? "切换亮色模式" : "切换暗色模式"}
        >
          {isDarkMode ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
        </button>

        {/* Page navigation icon */}
        {currentPage === "files" ? (
          <button
            onClick={() => onNavigate("settings")}
            className="flex h-9 w-9 items-center justify-center rounded-xl text-muted-foreground hover:text-foreground hover:bg-secondary transition-colors shrink-0"
            title="系统设置"
          >
            <Settings className="h-4 w-4" />
          </button>
        ) : (
          <button
            onClick={() => onNavigate("files")}
            className="flex h-9 w-9 items-center justify-center rounded-xl text-muted-foreground hover:text-foreground hover:bg-secondary transition-colors shrink-0"
            title="返回文件管理"
          >
            <Folder className="h-4 w-4" />
          </button>
        )}
      </div>
    </header>
  )
}

