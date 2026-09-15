import React, { useState, useEffect } from "react"
import {
  Folder,
  FileText,
  File,
  Trash2,
  ChevronRight,
  Home,
  PlayCircle,
  Loader2,
  Plus,
  Download,
  Edit2,
  X,
  Eye,
  Check,
  CheckCircle2,
} from "lucide-react"
import { api, VirtualFile } from "../lib/api"
import { formatBytes, formatDate } from "../lib/utils"
import { VideoPlayerModal } from "../components/VideoPlayerModal"

interface BreadcrumbItem {
  virtualID: string
  name: string
}

interface FilesPageProps {
  onOpenNewOffline?: () => void
  searchKeyword?: string
  onClearSearch?: () => void
  newTaskSubmitted?: number
}

interface ContextMenuState {
  visible: boolean
  x: number
  y: number
  file: VirtualFile | null
}

export const FilesPage: React.FC<FilesPageProps> = ({
  onOpenNewOffline,
  searchKeyword = "",
  onClearSearch,
  newTaskSubmitted = 0,
}) => {
  const [files, setFiles] = useState<VirtualFile[]>([])
  const [loading, setLoading] = useState(true)
  const [completeToast, setCompleteToast] = useState<string | null>(null)
  const knownActiveTasksRef = React.useRef<Map<string, string>>(new Map())
  const initialMountRef = React.useRef(true)
  const [breadcrumbs, setBreadcrumbs] = useState<BreadcrumbItem[]>([
    { virtualID: "root", name: "全部文件" },
  ])
  const [selectedIDs, setSelectedIDs] = useState<Set<string>>(new Set())
  const [isDeleting, setIsDeleting] = useState(false)

  // Context Menu state
  const [contextMenu, setContextMenu] = useState<ContextMenuState>({
    visible: false,
    x: 0,
    y: 0,
    file: null,
  })

  // Rename modal state
  const [renamingFile, setRenamingFile] = useState<VirtualFile | null>(null)
  const [newNameInput, setNewNameInput] = useState("")
  const [renameSubmitting, setRenameSubmitting] = useState(false)
  const [renameError, setRenameError] = useState("")

  // Video playback modal
  const [activeVideo, setActiveVideo] = useState<{ virtualID: string; name: string } | null>(null)

  // Image preview modal
  const [previewImage, setPreviewImage] = useState<{ url: string; name: string } | null>(null)

  const currentFolder = breadcrumbs[breadcrumbs.length - 1]

  const loadFiles = async (folderID: string) => {
    setLoading(true)
    setSelectedIDs(new Set())
    try {
      const data = await api.listFiles(folderID, "name", "asc")
      setFiles(data || [])
    } catch (err) {
      console.error("Failed to load files:", err)
    } finally {
      setLoading(false)
    }
  }

  // Effect for folder navigation or search
  useEffect(() => {
    if (searchKeyword && searchKeyword.trim()) {
      setLoading(true)
      setSelectedIDs(new Set())
      api
        .searchFiles(searchKeyword.trim())
        .then((res) => setFiles(res || []))
        .catch((err) => console.error("Search failed:", err))
        .finally(() => setLoading(false))
    } else {
      loadFiles(currentFolder.virtualID)
    }
  }, [currentFolder.virtualID, searchKeyword])

  // Watcher to detect task completion and auto-refresh files
  const checkTaskStatus = React.useCallback(async () => {
    try {
      const res = await api.listTasks("", 20, 0)
      const tasks = res?.tasks || []
      let completedFound = false
      let completedName = ""

      const currentActive = new Map<string, string>()

      for (const t of tasks) {
        if (t.status === "RUNNING" || t.status === "PENDING") {
          currentActive.set(t.id, t.status)
        } else if (t.status === "COMPLETE") {
          if (knownActiveTasksRef.current.has(t.id)) {
            completedFound = true
            completedName = t.file_name || "文件"
          }
        }
      }

      knownActiveTasksRef.current = currentActive

      if (completedFound && !initialMountRef.current) {
        setCompleteToast(`离线下载完成:「${completedName}」，已自动刷新文件列表`)
        // Auto refresh files in current folder
        try {
          const freshFiles = await api.listFiles(currentFolder.virtualID, "name", "asc")
          setFiles(freshFiles || [])
        } catch (e) {
          console.error("Auto refresh failed:", e)
        }
      }

      return currentActive.size > 0
    } catch {
      return false
    } finally {
      initialMountRef.current = false
    }
  }, [currentFolder.virtualID])

  // Polling loop: 2.5s when active tasks exist, 8s when idle
  useEffect(() => {
    let timer: any = null
    let active = true

    const poll = async () => {
      if (!active) return
      const hasActive = await checkTaskStatus()
      if (active) {
        timer = setTimeout(poll, hasActive ? 2500 : 8000)
      }
    }

    poll()

    return () => {
      active = false
      if (timer) clearTimeout(timer)
    }
  }, [checkTaskStatus])

  // Immediate check whenever user submits a new task
  useEffect(() => {
    if (newTaskSubmitted && newTaskSubmitted > 0) {
      checkTaskStatus()
      // Also reload files 1.5s later in case of instant cloud match (秒传)
      const timeout = setTimeout(async () => {
        try {
          const fresh = await api.listFiles(currentFolder.virtualID, "name", "asc")
          setFiles(fresh || [])
        } catch {}
      }, 1500)
      return () => clearTimeout(timeout)
    }
  }, [newTaskSubmitted, checkTaskStatus, currentFolder.virtualID])

  // Auto-dismiss complete toast after 4s
  useEffect(() => {
    if (completeToast) {
      const timer = setTimeout(() => setCompleteToast(null), 4000)
      return () => clearTimeout(timer)
    }
  }, [completeToast])

  // Context menu outside click & scroll listener
  useEffect(() => {
    const handleCloseMenu = () => {
      if (contextMenu.visible) {
        setContextMenu({ visible: false, x: 0, y: 0, file: null })
      }
    }
    window.addEventListener("click", handleCloseMenu)
    window.addEventListener("scroll", handleCloseMenu, true)
    return () => {
      window.removeEventListener("click", handleCloseMenu)
      window.removeEventListener("scroll", handleCloseMenu, true)
    }
  }, [contextMenu.visible])

  const handleContextMenu = (e: React.MouseEvent, file: VirtualFile) => {
    e.preventDefault()
    e.stopPropagation()
    const menuWidth = 160
    const menuHeight = 190
    const x = Math.min(e.clientX, window.innerWidth - menuWidth - 12)
    const y = Math.min(e.clientY, window.innerHeight - menuHeight - 12)
    setContextMenu({
      visible: true,
      x,
      y,
      file,
    })
  }

  const navigateToFolder = (virtualID: string, name: string) => {
    onClearSearch?.()
    setBreadcrumbs((prev) => [...prev, { virtualID, name }])
  }

  const navigateToBreadcrumb = (index: number) => {
    onClearSearch?.()
    setBreadcrumbs((prev) => prev.slice(0, index + 1))
  }

  const toggleSelect = (virtualID: string) => {
    const next = new Set(selectedIDs)
    if (next.has(virtualID)) {
      next.delete(virtualID)
    } else {
      next.add(virtualID)
    }
    setSelectedIDs(next)
  }

  // Delete directly without modal or confirmation prompt
  const handleDeleteSingle = async (virtualID: string, e?: React.MouseEvent) => {
    if (e) e.stopPropagation()
    try {
      await api.batchDelete([virtualID], true)
      setFiles((prev) => prev.filter((f) => f.virtual_id !== virtualID))
      setSelectedIDs((prev) => {
        const next = new Set(prev)
        next.delete(virtualID)
        return next
      })
    } catch (err: any) {
      console.error("Direct delete failed:", err)
    }
  }

  // Batch delete directly without modal or confirmation prompt
  const handleBatchDelete = async () => {
    if (selectedIDs.size === 0) return
    setIsDeleting(true)
    const ids = Array.from(selectedIDs)
    try {
      await api.batchDelete(ids, true)
      setFiles((prev) => prev.filter((f) => !selectedIDs.has(f.virtual_id)))
      setSelectedIDs(new Set())
    } catch (err: any) {
      console.error("Batch delete failed:", err)
    } finally {
      setIsDeleting(false)
    }
  }

  // Preview action
  const handlePreview = async (file: VirtualFile) => {
    setContextMenu({ visible: false, x: 0, y: 0, file: null })
    if (file.is_folder) {
      navigateToFolder(file.virtual_id, file.name)
      return
    }
    if (file.is_video) {
      setActiveVideo({ virtualID: file.virtual_id, name: file.name })
      return
    }
    // Image or other
    try {
      const info = await api.getPlaybackInfo(file.virtual_id)
      const url = info.direct_url || info.proxy_url
      if (file.mime_type.startsWith("image/") || file.thumbnail_link) {
        setPreviewImage({ url: url || file.thumbnail_link, name: file.name })
      } else if (url) {
        window.open(url, "_blank")
      }
    } catch {
      if (file.thumbnail_link) {
        setPreviewImage({ url: file.thumbnail_link, name: file.name })
      }
    }
  }

  // Direct download action
  const handleDownloadFile = async (file: VirtualFile) => {
    setContextMenu({ visible: false, x: 0, y: 0, file: null })
    try {
      const info = await api.getPlaybackInfo(file.virtual_id)
      const downloadUrl = info.direct_url || info.proxy_url
      if (downloadUrl) {
        const a = document.createElement("a")
        a.href = downloadUrl
        a.download = file.name
        a.target = "_blank"
        document.body.appendChild(a)
        a.click()
        document.body.removeChild(a)
      } else {
        alert("无法获取下载链接")
      }
    } catch (err: any) {
      alert("下载失败: " + (err.message || "未能解析下载直链"))
    }
  }

  // Start rename
  const handleStartRename = (file: VirtualFile) => {
    setContextMenu({ visible: false, x: 0, y: 0, file: null })
    setRenamingFile(file)
    setNewNameInput(file.name)
    setRenameError("")
  }

  // Confirm rename
  const handleConfirmRename = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!renamingFile || !newNameInput.trim()) return
    setRenameSubmitting(true)
    setRenameError("")
    try {
      await api.renameFile(renamingFile.virtual_id, newNameInput.trim())
      setFiles((prev) =>
        prev.map((f) =>
          f.virtual_id === renamingFile.virtual_id ? { ...f, name: newNameInput.trim() } : f
        )
      )
      setRenamingFile(null)
    } catch (err: any) {
      setRenameError(err.message || "重命名失败")
    } finally {
      setRenameSubmitting(false)
    }
  }

  return (
    <div className="space-y-4 relative min-h-[calc(100vh-8rem)]">
      {/* Breadcrumb Path & Search Filter Tag (Minimal Header) */}
      {(breadcrumbs.length > 1 || (searchKeyword && searchKeyword.trim())) && (
        <div className="flex items-center justify-between border-b pb-2.5">
          <div className="flex items-center gap-1.5 overflow-x-auto no-scrollbar py-0.5 text-xs sm:text-sm font-medium">
            {breadcrumbs.map((b, idx) => (
              <React.Fragment key={b.virtualID}>
                {idx > 0 && <ChevronRight className="h-4 w-4 text-muted-foreground shrink-0" />}
                <button
                  onClick={() => navigateToBreadcrumb(idx)}
                  className={`flex items-center gap-1 whitespace-nowrap rounded-lg px-2 py-1 transition-colors ${
                    idx === breadcrumbs.length - 1 && !searchKeyword
                      ? "font-semibold text-foreground bg-secondary/80"
                      : "text-muted-foreground hover:text-foreground hover:bg-secondary/40"
                  }`}
                >
                  {idx === 0 ? <Home className="h-3.5 w-3.5" /> : null}
                  <span>{b.name}</span>
                </button>
              </React.Fragment>
            ))}
          </div>

          {searchKeyword && searchKeyword.trim() && (
            <div className="flex items-center gap-1.5 bg-primary/10 text-primary text-xs px-2.5 py-1 rounded-full shrink-0">
              <span className="truncate max-w-[150px] sm:max-w-xs">搜索: "{searchKeyword}"</span>
              <button
                onClick={onClearSearch}
                className="hover:bg-primary/20 rounded p-0.5"
                title="清除搜索"
              >
                <X className="h-3 w-3" />
              </button>
            </div>
          )}
        </div>
      )}

      {/* Batch Actions Banner */}
      {selectedIDs.size > 0 && (
        <div className="flex items-center justify-between rounded-xl bg-primary/10 border border-primary/20 px-4 py-2 text-sm animate-in fade-in">
          <span className="font-medium text-primary text-xs sm:text-sm">
            已选择 {selectedIDs.size} 个项目 (直接删除，不进回收站)
          </span>
          <div className="flex items-center gap-2">
            <button
              onClick={() => setSelectedIDs(new Set())}
              className="px-2.5 py-1 rounded-lg hover:bg-primary/20 text-xs text-muted-foreground"
            >
              取消
            </button>
            <button
              onClick={handleBatchDelete}
              disabled={isDeleting}
              className="flex items-center gap-1.5 rounded-lg bg-destructive px-3 py-1 text-xs font-medium text-destructive-foreground shadow hover:bg-destructive/90 disabled:opacity-50"
            >
              {isDeleting ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Trash2 className="h-3.5 w-3.5" />}
              直接删除
            </button>
          </div>
        </div>
      )}

      {/* Grid View with Video / Image Covers */}
      {loading ? (
        <div className="flex flex-col items-center justify-center py-28 text-muted-foreground gap-3">
          <Loader2 className="h-8 w-8 animate-spin text-primary" />
          <span className="text-xs">加载网盘内容中...</span>
        </div>
      ) : files.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-28 text-muted-foreground">
          <Folder className="h-16 w-16 stroke-1 text-muted-foreground/30 mb-3" />
          <p className="text-base font-semibold text-foreground">暂无文件</p>
          <p className="text-xs text-muted-foreground mt-1">
            当前目录为空。点击右下角按钮即可新建离线下载任务！
          </p>
        </div>
      ) : (
        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-3.5 sm:gap-4">
          {files.map((file) => {
            const isSelected = selectedIDs.has(file.virtual_id)
            return (
              <div
                key={file.virtual_id}
                onContextMenu={(e) => handleContextMenu(e, file)}
                className={`group relative flex flex-col rounded-2xl border bg-card p-3 shadow-sm hover:border-primary/50 hover:shadow-md transition-all select-none ${
                  isSelected ? "ring-2 ring-primary bg-primary/5" : ""
                }`}
              >
                {/* Select Checkbox */}
                <div className="absolute top-2.5 left-2.5 z-20">
                  <input
                    type="checkbox"
                    checked={isSelected}
                    onChange={() => toggleSelect(file.virtual_id)}
                    className="rounded border-gray-300 h-4 w-4 cursor-pointer accent-primary"
                  />
                </div>

                {/* Direct Delete Button (Hover) */}
                <button
                  onClick={(e) => handleDeleteSingle(file.virtual_id, e)}
                  className="absolute top-2.5 right-2.5 z-20 p-1.5 rounded-lg bg-background/80 hover:bg-destructive hover:text-destructive-foreground text-muted-foreground opacity-0 group-hover:opacity-100 transition-all shadow-sm"
                  title="直接删除 (免确认，不进回收站)"
                >
                  <Trash2 className="h-3.5 w-3.5" />
                </button>

                {/* Media Thumbnail or Icon */}
                <div
                  onClick={() => {
                    if (file.is_folder) {
                      navigateToFolder(file.virtual_id, file.name)
                    } else if (file.is_video) {
                      setActiveVideo({ virtualID: file.virtual_id, name: file.name })
                    } else {
                      handlePreview(file)
                    }
                  }}
                  className="relative flex flex-col items-center justify-center h-32 sm:h-36 w-full cursor-pointer rounded-xl bg-secondary/30 overflow-hidden group-hover:bg-secondary/60 transition-all"
                >
                  {file.thumbnail_link ? (
                    <img
                      src={file.thumbnail_link}
                      alt={file.name}
                      className="w-full h-full object-cover group-hover:scale-105 transition-transform duration-300"
                      loading="lazy"
                      onError={(e) => {
                        (e.target as HTMLElement).style.display = "none"
                      }}
                    />
                  ) : null}

                  {/* Fallback Icon */}
                  {!file.thumbnail_link && (
                    file.is_folder ? (
                      <Folder className="h-12 w-12 sm:h-14 sm:w-14 text-amber-500 fill-amber-500/20" />
                    ) : file.is_video ? (
                      <PlayCircle className="h-12 w-12 sm:h-14 sm:w-14 text-primary" />
                    ) : (
                      <FileText className="h-12 w-12 sm:h-14 sm:w-14 text-muted-foreground" />
                    )
                  )}

                  {/* Video Play Overlay */}
                  {file.is_video && (
                    <div className="absolute inset-0 flex items-center justify-center bg-black/25 opacity-85 group-hover:opacity-100 transition-opacity">
                      <div className="flex h-10 w-10 sm:h-11 sm:w-11 items-center justify-center rounded-full bg-black/60 text-white shadow-lg backdrop-blur-sm group-hover:scale-110 transition-transform">
                        <PlayCircle className="h-6 w-6 sm:h-7 sm:w-7 text-primary" />
                      </div>
                    </div>
                  )}
                </div>

                {/* File Details */}
                <div className="mt-2.5 space-y-1">
                  <p
                    onClick={() => {
                      if (file.is_folder) {
                        navigateToFolder(file.virtual_id, file.name)
                      } else if (file.is_video) {
                        setActiveVideo({ virtualID: file.virtual_id, name: file.name })
                      } else {
                        handlePreview(file)
                      }
                    }}
                    className="font-medium text-xs truncate cursor-pointer hover:underline"
                    title={file.name}
                  >
                    {file.name}
                  </p>
                  <div className="flex items-center justify-between text-[11px] text-muted-foreground">
                    <span>{file.is_folder ? "文件夹" : formatBytes(file.size)}</span>
                    <span className="truncate max-w-[85px] bg-secondary/80 px-1.5 py-0.5 rounded text-[10px]" title={file.account_name}>
                      {file.account_name}
                    </span>
                  </div>
                </div>
              </div>
            )
          })}
        </div>
      )}

      {/* Floating Context Menu */}
      {contextMenu.visible && contextMenu.file && (
        <div
          style={{ top: `${contextMenu.y}px`, left: `${contextMenu.x}px` }}
          className="fixed z-50 min-w-[155px] rounded-2xl border bg-card/95 backdrop-blur-md shadow-2xl py-1.5 text-xs animate-in fade-in zoom-in-95"
          onClick={(e) => e.stopPropagation()}
        >
          {/* 预览 */}
          <button
            onClick={() => handlePreview(contextMenu.file!)}
            className="flex w-full items-center gap-2.5 px-3.5 py-2 text-left hover:bg-secondary transition-colors"
          >
            <Eye className="h-3.5 w-3.5 text-primary" />
            <span className="font-medium">预览</span>
          </button>

          {/* 下载 */}
          {!contextMenu.file.is_folder && (
            <button
              onClick={() => handleDownloadFile(contextMenu.file!)}
              className="flex w-full items-center gap-2.5 px-3.5 py-2 text-left hover:bg-secondary transition-colors"
            >
              <Download className="h-3.5 w-3.5 text-blue-500" />
              <span className="font-medium">下载</span>
            </button>
          )}

          {/* 重命名 */}
          <button
            onClick={() => handleStartRename(contextMenu.file!)}
            className="flex w-full items-center gap-2.5 px-3.5 py-2 text-left hover:bg-secondary transition-colors"
          >
            <Edit2 className="h-3.5 w-3.5 text-amber-500" />
            <span className="font-medium">重命名</span>
          </button>

          <div className="my-1 border-t" />

          {/* 直接删除 */}
          <button
            onClick={() => {
              const id = contextMenu.file!.virtual_id
              setContextMenu({ visible: false, x: 0, y: 0, file: null })
              handleDeleteSingle(id)
            }}
            className="flex w-full items-center gap-2.5 px-3.5 py-2 text-left text-destructive hover:bg-destructive/10 transition-colors"
          >
            <Trash2 className="h-3.5 w-3.5" />
            <span className="font-medium">删除</span>
          </button>
        </div>
      )}

      {/* Rename Modal */}
      {renamingFile && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4">
          <div className="w-full max-w-sm rounded-2xl border bg-card p-5 shadow-2xl animate-in zoom-in-95">
            <div className="flex items-center justify-between border-b pb-3 mb-4">
              <h3 className="font-semibold text-sm flex items-center gap-2">
                <Edit2 className="h-4 w-4 text-primary" />
                重命名项目
              </h3>
              <button
                onClick={() => setRenamingFile(null)}
                className="p-1 rounded-lg hover:bg-secondary text-muted-foreground"
              >
                <X className="h-4 w-4" />
              </button>
            </div>

            <form onSubmit={handleConfirmRename} className="space-y-4">
              <div>
                <label className="text-xs font-semibold text-muted-foreground">新名称</label>
                <input
                  type="text"
                  required
                  autoFocus
                  value={newNameInput}
                  onChange={(e) => setNewNameInput(e.target.value)}
                  className="mt-1.5 w-full rounded-xl border bg-background px-3 py-2 text-xs focus:outline-none focus:ring-2 focus:ring-primary"
                />
              </div>

              {renameError && <p className="text-xs text-destructive">{renameError}</p>}

              <div className="flex justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setRenamingFile(null)}
                  className="rounded-xl border px-3.5 py-1.5 text-xs font-medium hover:bg-secondary"
                >
                  取消
                </button>
                <button
                  type="submit"
                  disabled={renameSubmitting || !newNameInput.trim()}
                  className="flex items-center gap-1 rounded-xl bg-primary px-4 py-1.5 text-xs font-medium text-primary-foreground shadow hover:bg-primary/90 disabled:opacity-50"
                >
                  {renameSubmitting && <Loader2 className="h-3.5 w-3.5 animate-spin" />}
                  确认
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Image Preview Lightbox Modal */}
      {previewImage && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/85 backdrop-blur-md p-4 animate-in fade-in"
          onClick={() => setPreviewImage(null)}
        >
          <div className="relative max-w-4xl max-h-[90vh] flex flex-col items-center">
            <button
              onClick={() => setPreviewImage(null)}
              className="absolute -top-10 right-0 p-1.5 rounded-full bg-black/60 text-white hover:bg-black/80 transition-colors"
            >
              <X className="h-5 w-5" />
            </button>
            <img
              src={previewImage.url}
              alt={previewImage.name}
              className="max-h-[82vh] max-w-full rounded-xl object-contain shadow-2xl"
              onClick={(e) => e.stopPropagation()}
            />
            <p className="text-xs text-white/80 mt-2 text-center truncate max-w-md">{previewImage.name}</p>
          </div>
        </div>
      )}

      {/* Video Player Modal */}
      {activeVideo && (
        <VideoPlayerModal
          virtualID={activeVideo.virtualID}
          fileName={activeVideo.name}
          onClose={() => setActiveVideo(null)}
        />
      )}

      {/* Auto Refresh Floating Toast Notification */}
      {completeToast && (
        <div className="fixed bottom-16 sm:bottom-20 right-4 sm:right-6 z-50 flex items-center gap-2.5 rounded-2xl border border-emerald-500/40 bg-card/95 p-3.5 sm:px-4 sm:py-3 text-xs font-medium text-emerald-500 shadow-2xl backdrop-blur-md animate-in fade-in slide-in-from-bottom-5">
          <CheckCircle2 className="h-4 w-4 shrink-0 text-emerald-500 animate-pulse" />
          <span className="text-foreground">{completeToast}</span>
          <button
            onClick={() => setCompleteToast(null)}
            className="ml-2 text-muted-foreground hover:text-foreground p-0.5 rounded-lg hover:bg-secondary"
          >
            <X className="h-3.5 w-3.5" />
          </button>
        </div>
      )}

      {/* Floating Action Button (FAB) for New Offline Task */}
      {onOpenNewOffline && (
        <button
          onClick={onOpenNewOffline}
          className="fixed right-4 bottom-4 sm:right-6 sm:bottom-6 z-40 flex items-center gap-2 rounded-full bg-primary px-4 py-2.5 sm:px-5 sm:py-3 text-xs sm:text-sm font-semibold text-primary-foreground shadow-xl hover:bg-primary/90 hover:scale-105 active:scale-95 transition-all"
          title="新建离线下载"
        >
          <Plus className="h-4 w-4 sm:h-5 sm:w-5" />
          <span>新建下载</span>
        </button>
      )}
    </div>
  )
}

