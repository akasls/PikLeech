import React, { useState, useEffect, useRef } from "react"
import { X, Play, Pause, Volume2, VolumeX, Maximize, RotateCcw, ShieldCheck, Zap } from "lucide-react"
import { api, PlaybackInfo } from "../lib/api"
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
  const [isPlaying, setIsPlaying] = useState(false)
  const [isMuted, setIsMuted] = useState(false)
  const [playbackRate, setPlaybackRate] = useState(1)

  const videoRef = useRef<HTMLVideoElement>(null)

  useEffect(() => {
    let active = true
    setLoading(true)
    setError("")

    api
      .getPlaybackInfo(virtualID)
      .then((info) => {
        if (!active) return
        setPlaybackInfo(info)
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
  }, [virtualID])

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
  }, [loading, streamMode, virtualID])

  const handleTimeUpdate = () => {
    if (videoRef.current) {
      localStorage.setItem(`pikpak_progress_${virtualID}`, videoRef.current.currentTime.toString())
    }
  }

  const togglePlay = () => {
    if (!videoRef.current) return
    if (videoRef.current.paused) {
      videoRef.current.play()
      setIsPlaying(true)
    } else {
      videoRef.current.pause()
      setIsPlaying(false)
    }
  }

  const toggleMute = () => {
    if (!videoRef.current) return
    videoRef.current.muted = !videoRef.current.muted
    setIsMuted(videoRef.current.muted)
  }

  const changePlaybackRate = (rate: number) => {
    if (!videoRef.current) return
    videoRef.current.playbackRate = rate
    setPlaybackRate(rate)
  }

  const handleFullscreen = () => {
    if (videoRef.current) {
      if (videoRef.current.requestFullscreen) {
        videoRef.current.requestFullscreen()
      }
    }
  }

  const currentStreamSrc =
    streamMode === "proxy" ? playbackInfo?.proxy_url : playbackInfo?.direct_url

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-md p-2 md:p-6">
      <div className="flex flex-col w-full max-w-5xl max-h-[95vh] rounded-2xl border bg-card shadow-2xl overflow-hidden animate-in zoom-in-95">
        {/* Header */}
        <div className="flex items-center justify-between border-b px-5 py-3.5 bg-card/90">
          <div className="flex items-center gap-3 overflow-hidden">
            <h3 className="font-semibold text-base truncate max-w-md md:max-w-xl" title={fileName}>
              {fileName}
            </h3>
            {playbackInfo && (
              <span className="hidden sm:inline-block rounded-full bg-secondary px-2.5 py-0.5 text-xs text-muted-foreground whitespace-nowrap">
                {formatBytes(playbackInfo.size)} · 来源: {playbackInfo.account_name}
              </span>
            )}
          </div>
          <div className="flex items-center gap-2">
            {/* Mode toggle */}
            <div className="flex rounded-lg border bg-secondary p-0.5 text-xs font-medium">
              <button
                onClick={() => setStreamMode("proxy")}
                className={`flex items-center gap-1 px-2.5 py-1 rounded-md transition-all ${
                  streamMode === "proxy"
                    ? "bg-card text-foreground shadow-sm"
                    : "text-muted-foreground hover:text-foreground"
                }`}
                title="通过后端中转播放并穿透账号代理 (支持 Range 206 拖拽)"
              >
                <ShieldCheck className="h-3.5 w-3.5 text-primary" />
                Proxy 代理流
              </button>
              <button
                onClick={() => setStreamMode("direct")}
                className={`flex items-center gap-1 px-2.5 py-1 rounded-md transition-all ${
                  streamMode === "direct"
                    ? "bg-card text-foreground shadow-sm"
                    : "text-muted-foreground hover:text-foreground"
                }`}
                title="直连 PikPak CDN 媒体地址"
              >
                <Zap className="h-3.5 w-3.5 text-amber-500" />
                Direct 直连
              </button>
            </div>

            <button onClick={onClose} className="p-1.5 rounded-lg hover:bg-secondary">
              <X className="h-5 w-5" />
            </button>
          </div>
        </div>

        {/* Video Screen */}
        <div className="relative flex-1 bg-black flex items-center justify-center min-h-[300px] md:min-h-[500px]">
          {loading && (
            <div className="flex flex-col items-center gap-3 text-white/70">
              <div className="h-8 w-8 animate-spin rounded-full border-2 border-primary border-t-transparent" />
              <span className="text-sm">正在解析媒体地址与代理流...</span>
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
              onPlay={() => setIsPlaying(true)}
              onPause={() => setIsPlaying(false)}
              className="max-h-[70vh] w-full object-contain"
            />
          )}
        </div>

        {/* Footer controls & speed */}
        <div className="flex items-center justify-between border-t bg-card px-4 py-2.5 text-xs text-muted-foreground">
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

          <div className="flex items-center gap-2 text-muted-foreground text-xs">
            <span>已自动记录播放进度</span>
          </div>
        </div>
      </div>
    </div>
  )
}
