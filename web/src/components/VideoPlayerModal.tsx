import React, { useState, useEffect, useRef } from "react"
import {
  X,
  ShieldCheck,
  Zap,
  Copy,
  Check,
  ExternalLink,
  AlertTriangle,
  Tv,
  Film,
} from "lucide-react"
import { api, PlaybackInfo, MediaStreamItem } from "../lib/api"
import { formatBytes } from "../lib/utils"

interface VideoPlayerModalProps {
  virtualID: string
  fileName: string
  onClose: () => void
}

export const VideoPlayerModal: React.FC<VideoPlayerModalProps> = ({
  virtualID,
  fileName,
  onClose,
}) => {
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState("")
  const [playbackInfo, setPlaybackInfo] = useState<PlaybackInfo | null>(null)
  const [streamMode, setStreamMode] = useState<"proxy" | "direct">("direct")
  const [selectedMediaId, setSelectedMediaId] = useState<string>("")
  const [playbackRate, setPlaybackRate] = useState(1)
  const [copied, setCopied] = useState(false)
  const [playbackFormatError, setPlaybackFormatError] = useState(false)

  const videoRef = useRef<HTMLVideoElement>(null)

  useEffect(() => {
    let active = true
    setLoading(true)
    setError("")
    setPlaybackFormatError(false)

    api
      .getPlaybackInfo(virtualID)
      .then((info) => {
        if (!active) return
        setPlaybackInfo(info)

        // Select initial media
        if (info.medias && info.medias.length > 0) {
          const isMKV = info.is_mkv || fileName.toLowerCase().endsWith(".mkv")
          if (isMKV) {
            // For MKV, prioritize browser-friendly transcode streams
            const transcode = info.medias.find((m) => !m.is_origin)
            if (transcode) {
              setSelectedMediaId(transcode.media_id)
            } else {
              setSelectedMediaId(info.medias[0].media_id)
            }
          } else {
            // Default to origin or first
            const origin = info.medias.find((m) => m.is_origin)
            setSelectedMediaId(origin ? origin.media_id : info.medias[0].media_id)
          }
        }

        setLoading(false)
      })
      .catch((err) => {
        if (!active) return
        setError(err.message || "无法获取播放地址")
        setLoading(false)
      })

    return () => {
      active = false
    }
  }, [virtualID, fileName])

  // Restore saved progress
  useEffect(() => {
    if (!loading && videoRef.current) {
      const savedTime = localStorage.getItem(`pikpak_progress_${virtualID}`)
      if (savedTime) {
        const t = parseFloat(savedTime)
        if (!isNaN(t) && t > 0) {
          videoRef.current.currentTime = t
        }
      }
    }
  }, [loading, selectedMediaId, streamMode, virtualID])

  const handleTimeUpdate = () => {
    if (videoRef.current) {
      localStorage.setItem(
        `pikpak_progress_${virtualID}`,
        videoRef.current.currentTime.toString()
      )
    }
  }

  const changePlaybackRate = (rate: number) => {
    if (!videoRef.current) return
    videoRef.current.playbackRate = rate
    setPlaybackRate(rate)
  }

  // Find currently active stream item
  const activeMediaItem: MediaStreamItem | undefined = playbackInfo?.medias?.find(
    (m) => m.media_id === selectedMediaId
  )

  const currentStreamSrc =
    streamMode === "proxy"
      ? activeMediaItem?.proxy_url || playbackInfo?.proxy_url
      : activeMediaItem?.url || playbackInfo?.direct_url

  const fullPlayableUrl =
    streamMode === "proxy"
      ? `${window.location.origin}${currentStreamSrc}`
      : currentStreamSrc || ""

  const handleCopyLink = () => {
    if (!fullPlayableUrl) return
    navigator.clipboard.writeText(fullPlayableUrl).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    })
  }

  const handleOpenPlayer = (protocol: "potplayer" | "vlc" | "iina") => {
    if (!fullPlayableUrl) return
    let target = ""
    if (protocol === "potplayer") {
      target = `potplayer://${fullPlayableUrl}`
    } else if (protocol === "vlc") {
      target = `vlc://${fullPlayableUrl}`
    } else if (protocol === "iina") {
      target = `iina://weblink?url=${encodeURIComponent(fullPlayableUrl)}`
    }
    window.location.href = target
  }

  const isMKVFile = fileName.toLowerCase().endsWith(".mkv") || playbackInfo?.is_mkv

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-md p-2 md:p-6">
      <div className="flex flex-col w-full max-w-5xl max-h-[95vh] rounded-2xl border bg-card shadow-2xl overflow-hidden animate-in zoom-in-95">
        {/* Header */}
        <div className="flex flex-wrap items-center justify-between border-b px-4 py-3 bg-card/90 gap-2">
          <div className="flex items-center gap-2 overflow-hidden min-w-0">
            <Film className="h-4 w-4 text-primary shrink-0" />
            <h3
              className="font-semibold text-sm md:text-base truncate max-w-xs md:max-w-md"
              title={fileName}
            >
              {fileName}
            </h3>
            {playbackInfo && (
              <span className="hidden sm:inline-block rounded-full bg-secondary px-2.5 py-0.5 text-xs text-muted-foreground whitespace-nowrap">
                {formatBytes(playbackInfo.size)}
              </span>
            )}
          </div>

          <div className="flex items-center gap-2 flex-wrap">
            {/* Resolution Selector */}
            {playbackInfo?.medias && playbackInfo.medias.length > 1 && (
              <div className="flex items-center rounded-lg border bg-secondary/60 p-0.5 text-xs font-medium">
                {playbackInfo.medias.map((m) => (
                  <button
                    key={m.media_id}
                    onClick={() => {
                      setSelectedMediaId(m.media_id)
                      setPlaybackFormatError(false)
                    }}
                    className={`px-2 py-1 rounded-md transition-all ${
                      selectedMediaId === m.media_id
                        ? "bg-primary text-primary-foreground font-semibold shadow-xs"
                        : "text-muted-foreground hover:text-foreground"
                    }`}
                    title={m.media_name || m.resolution_name}
                  >
                    {m.resolution_name || (m.is_origin ? "原画" : "高清")}
                  </button>
                ))}
              </div>
            )}

            {/* Mode toggle */}
            <div className="flex rounded-lg border bg-secondary/60 p-0.5 text-xs font-medium">
              <button
                onClick={() => setStreamMode("direct")}
                className={`flex items-center gap-1 px-2.5 py-1 rounded-md transition-all ${
                  streamMode === "direct"
                    ? "bg-card text-foreground shadow-xs"
                    : "text-muted-foreground hover:text-foreground"
                }`}
                title="直连 PikPak CDN 媒体地址"
              >
                <Zap className="h-3.5 w-3.5 text-amber-500" />
                <span className="hidden sm:inline">CDN</span> 直连
              </button>
              <button
                onClick={() => setStreamMode("proxy")}
                className={`flex items-center gap-1 px-2.5 py-1 rounded-md transition-all ${
                  streamMode === "proxy"
                    ? "bg-card text-foreground shadow-xs"
                    : "text-muted-foreground hover:text-foreground"
                }`}
                title="通过中转穿透账号代理 (支持 Range 206 拖拽)"
              >
                <ShieldCheck className="h-3.5 w-3.5 text-primary" />
                <span className="hidden sm:inline">Proxy</span> 代理
              </button>
            </div>

            <button
              onClick={onClose}
              className="p-1.5 rounded-lg hover:bg-secondary text-muted-foreground hover:text-foreground"
            >
              <X className="h-5 w-5" />
            </button>
          </div>
        </div>

        {/* Video Screen */}
        <div className="relative flex-1 bg-black flex items-center justify-center min-h-[300px] md:min-h-[500px]">
          {loading && (
            <div className="flex flex-col items-center gap-3 text-white/70">
              <div className="h-8 w-8 animate-spin rounded-full border-2 border-primary border-t-transparent" />
              <span className="text-sm">正在解析媒体地址与清晰度流...</span>
            </div>
          )}

          {error && (
            <div className="text-center p-6 text-destructive">
              <p className="font-semibold">{error}</p>
              <button
                onClick={onClose}
                className="mt-4 rounded-lg bg-secondary px-4 py-1.5 text-sm font-medium text-foreground hover:bg-secondary/80"
              >
                关闭
              </button>
            </div>
          )}

          {!loading && !error && currentStreamSrc && (
            <video
              ref={videoRef}
              src={currentStreamSrc}
              controls
              autoPlay
              playsInline
              onTimeUpdate={handleTimeUpdate}
              onError={() => setPlaybackFormatError(true)}
              className="max-h-[70vh] w-full object-contain"
            />
          )}

          {/* Browser MKV / Format Incompatibility Notice Overlay */}
          {playbackFormatError && (
            <div className="absolute inset-x-4 bottom-14 md:bottom-16 z-20 flex flex-col items-center gap-3 rounded-xl bg-black/90 border border-amber-500/50 p-4 text-center backdrop-blur-md shadow-2xl animate-in fade-in slide-in-from-bottom-3">
              <div className="flex items-center gap-2 text-amber-400 text-sm font-medium">
                <AlertTriangle className="h-4 w-4 shrink-0" />
                <span>
                  当前视频格式（如 MKV 封装或特殊音轨）可能无法在浏览器原生直接硬解
                </span>
              </div>
              <p className="text-xs text-white/70 max-w-lg">
                建议切换上方转码清晰度（如 1080P/720P MP4），或使用外部专业播放器（支持全部 MKV、4K HDR 与多音轨解码）：
              </p>
              <div className="flex flex-wrap items-center justify-center gap-2">
                <button
                  onClick={() => handleOpenPlayer("potplayer")}
                  className="flex items-center gap-1.5 rounded-lg bg-amber-500/20 border border-amber-500/40 px-3 py-1 text-xs text-amber-300 hover:bg-amber-500/30 transition-all"
                >
                  <Tv className="h-3.5 w-3.5" />
                  PotPlayer 播放
                </button>
                <button
                  onClick={() => handleOpenPlayer("vlc")}
                  className="flex items-center gap-1.5 rounded-lg bg-orange-500/20 border border-orange-500/40 px-3 py-1 text-xs text-orange-300 hover:bg-orange-500/30 transition-all"
                >
                  <ExternalLink className="h-3.5 w-3.5" />
                  VLC 播放
                </button>
                <button
                  onClick={() => handleOpenPlayer("iina")}
                  className="flex items-center gap-1.5 rounded-lg bg-blue-500/20 border border-blue-500/40 px-3 py-1 text-xs text-blue-300 hover:bg-blue-500/30 transition-all"
                >
                  <ExternalLink className="h-3.5 w-3.5" />
                  IINA 播放 (Mac)
                </button>
                <button
                  onClick={handleCopyLink}
                  className="flex items-center gap-1.5 rounded-lg bg-secondary px-3 py-1 text-xs text-foreground hover:bg-secondary/80 transition-all"
                >
                  {copied ? (
                    <Check className="h-3.5 w-3.5 text-emerald-500" />
                  ) : (
                    <Copy className="h-3.5 w-3.5" />
                  )}
                  {copied ? "已复制直链" : "复制直链"}
                </button>
              </div>
            </div>
          )}
        </div>

        {/* Footer controls & speed & external launch */}
        <div className="flex flex-wrap items-center justify-between border-t bg-card px-4 py-2.5 text-xs text-muted-foreground gap-2">
          <div className="flex items-center gap-2">
            <span>倍速:</span>
            {[0.75, 1, 1.25, 1.5, 2].map((r) => (
              <button
                key={r}
                onClick={() => changePlaybackRate(r)}
                className={`px-2 py-0.5 rounded ${
                  playbackRate === r
                    ? "bg-primary text-primary-foreground font-semibold"
                    : "hover:bg-secondary text-foreground"
                }`}
              >
                {r}x
              </button>
            ))}
          </div>

          <div className="flex items-center gap-2 flex-wrap">
            {/* External Player Quick Launch */}
            <span className="hidden md:inline text-muted-foreground">外部调用:</span>
            <button
              onClick={() => handleOpenPlayer("potplayer")}
              className="px-2 py-1 rounded bg-secondary hover:bg-secondary/80 text-foreground transition-all flex items-center gap-1"
              title="一键调起 PotPlayer 播放"
            >
              PotPlayer
            </button>
            <button
              onClick={() => handleOpenPlayer("vlc")}
              className="px-2 py-1 rounded bg-secondary hover:bg-secondary/80 text-foreground transition-all flex items-center gap-1"
              title="一键调起 VLC 播放"
            >
              VLC
            </button>
            <button
              onClick={() => handleOpenPlayer("iina")}
              className="hidden sm:inline-flex px-2 py-1 rounded bg-secondary hover:bg-secondary/80 text-foreground transition-all items-center gap-1"
              title="一键调起 Mac IINA 播放"
            >
              IINA
            </button>
            <button
              onClick={handleCopyLink}
              className="px-2.5 py-1 rounded border hover:bg-secondary text-foreground transition-all flex items-center gap-1 font-medium"
              title="复制媒体播放直链 (适用于 Infuse, Kodi, MXPlayer 等)"
            >
              {copied ? (
                <>
                  <Check className="h-3.5 w-3.5 text-emerald-500" />
                  <span className="text-emerald-500">已复制</span>
                </>
              ) : (
                <>
                  <Copy className="h-3.5 w-3.5" />
                  <span>复制直链</span>
                </>
              )}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
