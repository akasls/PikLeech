import React, { useState, useEffect } from "react"
import {
  Folder,
  FileVideo,
  FileText,
  File,
  Search,
  LayoutGrid,
  List,
  Trash2,
  RefreshCw,
  ChevronRight,
  Home,
  PlayCircle,
  AlertCircle,
  Loader2,
} from "lucide-react"
import { api, VirtualFile } from "../lib/api"
import { formatBytes, formatDate } from "../lib/utils"
import { VideoPlayerModal } from "../components/VideoPlayerModal"

interface BreadcrumbItem {
  virtualID: string
  name: string
}

export const FilesPage: React.FC = () => {
  const [files, setFiles] = useState<VirtualFile[]>([])
  const [loading, setLoading] = useState(true)
  const [breadcrumbs, setBreadcrumbs] = useState<BreadcrumbItem[]>([
    { virtualID: "root", name: "全部文件" },
  ])
  const [viewMode, setViewMode] = useState<"list" | "grid">("list")
  const [searchKeyword, setSearchKeyword] = useState("")
  const [isSearching, setIsSearching] = useState(false)
  const [sortBy, setSortBy] = useState<"name" | "size" | "modified">("name")
  const [sortOrder, setSortOrder] = useState<"asc" | "desc">("asc")
  const [selectedIDs, setSelectedIDs] = useState<Set<string>>(new Set())

  // Video playback modal
  const [activeVideo, setActiveVideo] = useState<{ virtualID: string; name: string } | null>(null)

  // Batch delete modal
  const [showDeleteModal, setShowDeleteModal] = useState(false)
  const [isDeleting, setIsDeleting] = useState(false)
  const [permanentDelete, setPermanentDelete] = useState(false)
  const [deleteSummary, setDeleteSummary] = useState<any>(null)

  const currentFolder = breadcrumbs[breadcrumbs.length - 1]

  const loadFiles = async (folderID: string) => {
    setLoading(true)
    setSelectedIDs(new Set())
    try {
      const data = await api.listFiles(folderID, sortBy, sortOrder)
      setFiles(data || [])
    } catch (err) {
      console.error(err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (!isSearching) {
      loadFiles(currentFolder.virtualID)
    }
  }, [currentFolder.virtualID, sortBy, sortOrder])

  const handleSearch = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!searchKeyword.trim()) {
      setIsSearching(false)
      loadFiles(currentFolder.virtualID)
      return
    }

    setLoading(true)
    setIsSearching(true)
    try {
      const results = await api.searchFiles(searchKeyword)
      setFiles(results || [])
    } catch (err) {
      console.error(err)
    } finally {
      setLoading(false)
    }
  }

  const clearSearch = () => {
    setSearchKeyword("")
    setIsSearching(false)
    loadFiles(currentFolder.virtualID)
  }

  const navigateToFolder = (virtualID: string, name: string) => {
    setIsSearching(false)
    setSearchKeyword("")
    setBreadcrumbs((prev) => [...prev, { virtualID, name }])
  }

  const navigateToBreadcrumb = (index: number) => {
    setIsSearching(false)
    setSearchKeyword("")
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

  const toggleSelectAll = () => {
    if (selectedIDs.size === files.length) {
      setSelectedIDs(new Set())
    } else {
      setSelectedIDs(new Set(files.map((f) => f.virtual_id)))
    }
  }

  const handleBatchDelete = async () => {
    if (selectedIDs.size === 0) return
    setIsDeleting(true)
    setDeleteSummary(null)

    try {
      const ids = Array.from(selectedIDs)
      const res = await api.batchDelete(ids, permanentDelete)
      setDeleteSummary(res)
      loadFiles(currentFolder.virtualID)
      setSelectedIDs(new Set())
    } catch (err: any) {
      alert("批量删除失败: " + err.message)
    } finally {
      setIsDeleting(false)
    }
  }

  return (
    <div className="space-y-4">
      {/* Action Bar */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 border-b pb-4">
        {/* Breadcrumb path */}
        <div className="flex items-center gap-1.5 overflow-x-auto py-1 text-sm font-medium">
          {breadcrumbs.map((b, idx) => (
            <React.Fragment key={b.virtualID}>
              {idx > 0 && <ChevronRight className="h-4 w-4 text-muted-foreground shrink-0" />}
              <button
                onClick={() => navigateToBreadcrumb(idx)}
                className={`flex items-center gap-1 whitespace-nowrap rounded-lg px-2 py-1 transition-colors ${
                  idx === breadcrumbs.length - 1
                    ? "font-semibold text-foreground bg-secondary/80"
                    : "text-muted-foreground hover:text-foreground hover:bg-secondary/40"
                }`}
              >
                {idx === 0 ? <Home className="h-3.5 w-3.5" /> : null}
                <span>{b.name}</span>
              </button>
            </React.Fragment>
          ))}
          {isSearching && (
            <span className="text-xs text-primary font-normal bg-primary/10 px-2 py-0.5 rounded-full ml-2">
              搜索结果: "{searchKeyword}" (
              <button onClick={clearSearch} className="underline">
                清除
              </button>
              )
            </span>
          )}
        </div>

        {/* Right side controls: Search, Sort, View mode */}
        <div className="flex items-center gap-2">
          {/* Search bar */}
          <form onSubmit={handleSearch} className="relative">
            <Search className="absolute left-3 top-2.5 h-4 w-4 text-muted-foreground" />
            <input
              type="text"
              placeholder="搜索跨账号文件..."
              value={searchKeyword}
              onChange={(e) => setSearchKeyword(e.target.value)}
              className="h-9 w-44 sm:w-60 rounded-xl border bg-background pl-9 pr-3 text-xs focus:outline-none focus:ring-2 focus:ring-primary"
            />
          </form>

          {/* Sort dropdown */}
          <select
            value={`${sortBy}-${sortOrder}`}
            onChange={(e) => {
              const [f, o] = e.target.value.split("-")
              setSortBy(f as any)
              setSortOrder(o as any)
            }}
            className="h-9 rounded-xl border bg-card px-2.5 text-xs text-foreground focus:outline-none"
          >
            <option value="name-asc">按名称 (A-Z)</option>
            <option value="name-desc">按名称 (Z-A)</option>
            <option value="size-desc">按大小 (大到小)</option>
            <option value="size-asc">按大小 (小到大)</option>
            <option value="modified-desc">按修改时间 (新到旧)</option>
            <option value="modified-asc">按修改时间 (旧到新)</option>
          </select>

          {/* View toggle */}
          <div className="flex rounded-xl border bg-secondary p-0.5">
            <button
              onClick={() => setViewMode("list")}
              className={`p-1.5 rounded-lg ${viewMode === "list" ? "bg-card shadow-sm" : "text-muted-foreground"}`}
              title="列表视图"
            >
              <List className="h-4 w-4" />
            </button>
            <button
              onClick={() => setViewMode("grid")}
              className={`p-1.5 rounded-lg ${viewMode === "grid" ? "bg-card shadow-sm" : "text-muted-foreground"}`}
              title="网格视图"
            >
              <LayoutGrid className="h-4 w-4" />
            </button>
          </div>

          <button
            onClick={() => loadFiles(currentFolder.virtualID)}
            className="p-2 rounded-xl border hover:bg-secondary text-muted-foreground hover:text-foreground"
            title="刷新"
          >
            <RefreshCw className={`h-4 w-4 ${loading ? "animate-spin" : ""}`} />
          </button>
        </div>
      </div>

      {/* Batch Actions Banner */}
      {selectedIDs.size > 0 && (
        <div className="flex items-center justify-between rounded-xl bg-primary/10 border border-primary/20 px-4 py-2.5 text-sm">
          <span className="font-medium text-primary">已选择 {selectedIDs.size} 个项目 (支持跨账号)</span>
          <div className="flex items-center gap-2">
            <button
              onClick={() => setSelectedIDs(new Set())}
              className="px-3 py-1 rounded-lg hover:bg-primary/20 text-xs text-muted-foreground"
            >
              取消全选
            </button>
            <button
              onClick={() => {
                setDeleteSummary(null)
                setShowDeleteModal(true)
              }}
              className="flex items-center gap-1.5 rounded-lg bg-destructive px-3 py-1.5 text-xs font-medium text-destructive-foreground shadow hover:bg-destructive/90"
            >
              <Trash2 className="h-3.5 w-3.5" />
              批量删除
            </button>
          </div>
        </div>
      )}

      {/* Content Area */}
      {loading ? (
        <div className="flex flex-col items-center justify-center py-20 text-muted-foreground gap-3">
          <Loader2 className="h-8 w-8 animate-spin text-primary" />
          <span className="text-sm">正在聚合跨账号文件列表...</span>
        </div>
      ) : files.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-24 text-muted-foreground">
          <Folder className="h-16 w-16 stroke-1 text-muted-foreground/40 mb-3" />
          <p className="text-base font-semibold text-foreground">暂无文件</p>
          <p className="text-xs text-muted-foreground mt-1">
            当前目录为空，或者还没有账号同步文件。通过右上角新建离线下载开始！
          </p>
        </div>
      ) : viewMode === "list" ? (
        /* List View */
        <div className="rounded-2xl border bg-card overflow-hidden shadow-sm">
          <table className="w-full text-left text-sm">
            <thead className="border-b bg-secondary/40 text-xs font-semibold text-muted-foreground">
              <tr>
                <th className="w-10 px-4 py-3">
                  <input
                    type="checkbox"
                    checked={selectedIDs.size === files.length && files.length > 0}
                    onChange={toggleSelectAll}
                    className="rounded border-gray-300"
                  />
                </th>
                <th className="px-4 py-3">文件名</th>
                <th className="px-4 py-3 w-32">大小</th>
                <th className="px-4 py-3 w-36">来源账号</th>
                <th className="px-4 py-3 w-40">修改时间</th>
              </tr>
            </thead>
            <tbody className="divide-y">
              {files.map((file) => {
                const isSelected = selectedIDs.has(file.virtual_id)
                return (
                  <tr
                    key={file.virtual_id}
                    className={`hover:bg-secondary/30 transition-colors ${isSelected ? "bg-primary/5" : ""}`}
                  >
                    <td className="px-4 py-3">
                      <input
                        type="checkbox"
                        checked={isSelected}
                        onChange={() => toggleSelect(file.virtual_id)}
                        className="rounded border-gray-300"
                      />
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-3">
                        {file.is_folder ? (
                          <div
                            onClick={() => navigateToFolder(file.virtual_id, file.name)}
                            className="cursor-pointer text-amber-500 hover:text-amber-600"
                          >
                            <Folder className="h-5 w-5 fill-amber-500/20" />
                          </div>
                        ) : file.is_video ? (
                          <div
                            onClick={() => setActiveVideo({ virtualID: file.virtual_id, name: file.name })}
                            className="cursor-pointer text-primary hover:text-primary/80"
                            title="点击在线播放"
                          >
                            <PlayCircle className="h-5 w-5" />
                          </div>
                        ) : (
                          <div className="text-muted-foreground">
                            <File className="h-5 w-5" />
                          </div>
                        )}

                        <span
                          onClick={() => {
                            if (file.is_folder) {
                              navigateToFolder(file.virtual_id, file.name)
                            } else if (file.is_video) {
                              setActiveVideo({ virtualID: file.virtual_id, name: file.name })
                            }
                          }}
                          className={`font-medium truncate max-w-sm md:max-w-md cursor-pointer hover:underline ${
                            file.is_video ? "text-primary" : ""
                          }`}
                          title={file.name}
                        >
                          {file.name}
                        </span>
                      </div>
                    </td>
                    <td className="px-4 py-3 text-xs text-muted-foreground font-mono">
                      {file.is_folder ? "-" : formatBytes(file.size)}
                    </td>
                    <td className="px-4 py-3">
                      <span className="inline-block rounded-full bg-secondary px-2.5 py-0.5 text-xs text-muted-foreground">
                        {file.account_name}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-xs text-muted-foreground">
                      {formatDate(file.modified_time)}
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      ) : (
        /* Grid View */
        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 gap-3.5">
          {files.map((file) => {
            const isSelected = selectedIDs.has(file.virtual_id)
            return (
              <div
                key={file.virtual_id}
                className={`group relative flex flex-col rounded-2xl border bg-card p-3.5 shadow-sm hover:border-primary/50 transition-all ${
                  isSelected ? "ring-2 ring-primary bg-primary/5" : ""
                }`}
              >
                <div className="absolute top-3 left-3 z-10">
                  <input
                    type="checkbox"
                    checked={isSelected}
                    onChange={() => toggleSelect(file.virtual_id)}
                    className="rounded border-gray-300"
                  />
                </div>

                <div
                  onClick={() => {
                    if (file.is_folder) {
                      navigateToFolder(file.virtual_id, file.name)
                    } else if (file.is_video) {
                      setActiveVideo({ virtualID: file.virtual_id, name: file.name })
                    }
                  }}
                  className="flex flex-col items-center justify-center h-28 cursor-pointer rounded-xl bg-secondary/30 group-hover:bg-secondary/60 transition-colors"
                >
                  {file.is_folder ? (
                    <Folder className="h-12 w-12 text-amber-500 fill-amber-500/20" />
                  ) : file.is_video ? (
                    <PlayCircle className="h-12 w-12 text-primary" />
                  ) : (
                    <FileText className="h-12 w-12 text-muted-foreground" />
                  )}
                </div>

                <div className="mt-2.5 space-y-1">
                  <p
                    onClick={() => {
                      if (file.is_folder) {
                        navigateToFolder(file.virtual_id, file.name)
                      } else if (file.is_video) {
                        setActiveVideo({ virtualID: file.virtual_id, name: file.name })
                      }
                    }}
                    className="font-medium text-xs truncate cursor-pointer hover:underline"
                    title={file.name}
                  >
                    {file.name}
                  </p>
                  <div className="flex items-center justify-between text-[11px] text-muted-foreground">
                    <span>{file.is_folder ? "文件夹" : formatBytes(file.size)}</span>
                    <span className="truncate max-w-[80px]" title={file.account_name}>
                      {file.account_name}
                    </span>
                  </div>
                </div>
              </div>
            )
          })}
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

      {/* Batch Delete Confirmation Modal */}
      {showDeleteModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4">
          <div className="w-full max-w-md rounded-2xl border bg-card p-6 shadow-2xl">
            <h3 className="text-lg font-semibold flex items-center gap-2 text-destructive">
              <Trash2 className="h-5 w-5" />
              确认批量删除文件
            </h3>
            <p className="text-sm text-muted-foreground mt-2">
              您已选中来自不同 PikPak 账号的{" "}
              <strong className="text-foreground">{selectedIDs.size}</strong>{" "}
              个项目。后端将自动按账号分组调用对应 API 执行删除。
            </p>

            <div className="mt-4 flex items-center gap-2">
              <input
                type="checkbox"
                id="permDelete"
                checked={permanentDelete}
                onChange={(e) => setPermanentDelete(e.target.checked)}
                className="rounded border-gray-300"
              />
              <label htmlFor="permDelete" className="text-xs text-foreground cursor-pointer">
                彻底永久删除 (不勾选则移入 PikPak 回收站)
              </label>
            </div>

            {deleteSummary && (
              <div className="mt-4 rounded-xl border bg-secondary/50 p-3 text-xs space-y-1">
                <div className="font-semibold text-foreground">
                  删除结果: 成功 {deleteSummary.success} 个，失败 {deleteSummary.failed} 个
                </div>
                {deleteSummary.items
                  ?.filter((i: any) => !i.success)
                  .map((item: any, idx: number) => (
                    <div key={idx} className="text-destructive truncate">
                      {item.virtual_id}: {item.error}
                    </div>
                  ))}
              </div>
            )}

            <div className="mt-6 flex items-center justify-end gap-3">
              <button
                type="button"
                onClick={() => setShowDeleteModal(false)}
                className="rounded-xl border px-4 py-2 text-sm font-medium hover:bg-secondary"
              >
                {deleteSummary ? "完成" : "取消"}
              </button>
              {!deleteSummary && (
                <button
                  type="button"
                  onClick={handleBatchDelete}
                  disabled={isDeleting}
                  className="flex items-center gap-2 rounded-xl bg-destructive px-5 py-2 text-sm font-medium text-destructive-foreground shadow hover:bg-destructive/90 disabled:opacity-50"
                >
                  {isDeleting ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
                  确认删除
                </button>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
