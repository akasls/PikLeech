package stream

import (
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"pikpak-manager/internal/fileagg"
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

	mediaURL, err := client.GetMediaURL(c.Request.Context(), vf.PikPakFileID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to get media URL: %v", err)})
		return
	}

	proxyStreamURL := fmt.Sprintf("/api/media/stream/%s", virtualID)

	c.JSON(http.StatusOK, gin.H{
		"virtual_id":   vf.VirtualID,
		"name":         vf.Name,
		"size":         vf.Size,
		"mime_type":    vf.MimeType,
		"is_video":     vf.IsVideo,
		"direct_url":   mediaURL,
		"proxy_url":    proxyStreamURL,
		"account_name": vf.AccountName,
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

	mediaURL, err := client.GetMediaURL(c.Request.Context(), vf.PikPakFileID)
	if err != nil {
		c.String(http.StatusBadGateway, "failed to resolve upstream media link: %v", err)
		return
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
