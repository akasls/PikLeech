package stream

import (
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"pikpak-manager/internal/fileagg"
	"pikpak-manager/internal/pikpak"
)

type Handler struct {
	fileService *fileagg.Service
}

func NewHandler(fileSvc *fileagg.Service) *Handler {
	return &Handler{
		fileService: fileSvc,
	}
}

// GetPlaybackInfo returns playback mode, direct media URL, filename, and resolution
func (h *Handler) GetPlaybackInfo(c *gin.Context) {
	virtualID := c.Param("virtual_id")
	if virtualID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "virtual_id is required"})
		return
	}

	vf, client, err := h.fileService.GetVirtualFile(c.Request.Context(), virtualID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("file not found: %v", err)})
		return
	}

	details, err := client.GetPlaybackMediaDetails(c.Request.Context(), vf.PikPakFileID, vf.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to get media URL: %v", err)})
		return
	}

	proxyStreamURL := fmt.Sprintf("/api/media/stream/%s", virtualID)

	type StreamItem struct {
		pikpak.PlayableMedia
		ProxyURL string `json:"proxy_url"`
	}

	var streamItems []StreamItem
	for _, m := range details.Medias {
		streamItems = append(streamItems, StreamItem{
			PlayableMedia: m,
			ProxyURL:      fmt.Sprintf("/api/media/stream/%s?media_id=%s", virtualID, m.MediaID),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"virtual_id":   vf.VirtualID,
		"name":         vf.Name,
		"size":         vf.Size,
		"mime_type":    vf.MimeType,
		"is_video":     vf.IsVideo,
		"is_mkv":       details.IsMKV,
		"direct_url":   details.DirectURL,
		"origin_url":   details.OriginURL,
		"proxy_url":    proxyStreamURL,
		"account_name": vf.AccountName,
		"medias":       streamItems,
	})
}

// ProxyStream streams media with HTTP Range 206 through the account's configured proxy
func (h *Handler) ProxyStream(c *gin.Context) {
	virtualID := c.Param("virtual_id")
	if virtualID == "" {
		c.String(http.StatusBadRequest, "virtual_id required")
		return
	}

	vf, client, err := h.fileService.GetVirtualFile(c.Request.Context(), virtualID)
	if err != nil {
		c.String(http.StatusNotFound, "file not found: %v", err)
		return
	}

	details, err := client.GetPlaybackMediaDetails(c.Request.Context(), vf.PikPakFileID, vf.Name)
	if err != nil {
		c.String(http.StatusBadGateway, "failed to resolve upstream media link: %v", err)
		return
	}

	targetMediaID := c.Query("media_id")
	mediaURL := details.DirectURL
	if targetMediaID != "" {
		for _, m := range details.Medias {
			if m.MediaID == targetMediaID {
				mediaURL = m.URL
				break
			}
		}
	}

	rangeHdr := c.GetHeader("Range")
	upstreamResp, err := client.OpenMediaStream(c.Request.Context(), mediaURL, rangeHdr)
	if err != nil {
		c.String(http.StatusBadGateway, "failed to connect to upstream stream: %v", err)
		return
	}
	defer upstreamResp.Body.Close()

	// Forward critical streaming headers
	if ct := upstreamResp.Header.Get("Content-Type"); ct != "" {
		c.Header("Content-Type", ct)
	} else {
		c.Header("Content-Type", "video/mp4")
	}

	if cl := upstreamResp.Header.Get("Content-Length"); cl != "" {
		c.Header("Content-Length", cl)
	}

	if cr := upstreamResp.Header.Get("Content-Range"); cr != "" {
		c.Header("Content-Range", cr)
	}

	c.Header("Accept-Ranges", "bytes")

	c.Status(upstreamResp.StatusCode)

	// Stream body chunks to client
	_, _ = io.Copy(c.Writer, upstreamResp.Body)
}
