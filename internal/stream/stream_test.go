package stream

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"

	"pikpak-manager/internal/account"
	"pikpak-manager/internal/config"
	"pikpak-manager/internal/db"
	"pikpak-manager/internal/fileagg"
)

func TestStream_ProxyStreamRange(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tempDir, err := os.MkdirTemp("", "pikpak_test_stream_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg := &config.Config{
		DBPath:        filepath.Join(tempDir, "test.db"),
		AppSecret:     "secret-32-chars-for-stream-test!",
		AdminUsername: "admin",
		AdminPassword: "adminpassword",
	}

	database, err := db.InitDB(cfg)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer database.Close()

	accSvc := account.NewService(database, cfg.AppSecret)
	fileSvc := fileagg.NewService(database, accSvc)
	handler := NewHandler(fileSvc)

	r := gin.New()
	r.GET("/api/media/stream/:virtual_id", handler.ProxyStream)

	// Verify handler exists and handles bad request on non-existent file
	req := httptest.NewRequest(http.MethodGet, "/api/media/stream/invalid_vid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for invalid virtual id, got %d", w.Code)
	}
}
