package pikpak

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPikPak_ErrorClassification(t *testing.T) {
	// Quota Exceeded error test
	quotaJSON := []byte(`{"error":"task_daily_create_limit","error_description":"daily task limit reached"}`)
	err := ClassifyError(nil, 400, quotaJSON)
	if !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("Expected ErrQuotaExceeded, got %v", err)
	}

	// 429 Rate Limited test
	err429 := ClassifyError(nil, 429, []byte("Too many requests"))
	if !errors.Is(err429, ErrRateLimited) {
		t.Fatalf("Expected ErrRateLimited, got %v", err429)
	}

	// 401 Auth Failed test
	err401 := ClassifyError(nil, 401, []byte(`{"error":"unauthorized"}`))
	if !errors.Is(err401, ErrAuthFailed) {
		t.Fatalf("Expected ErrAuthFailed, got %v", err401)
	}

	// 4126 Refresh token expired
	err4126 := ClassifyError(nil, 400, []byte(`{"error_code":4126,"error":"invalid_refresh_token"}`))
	if !errors.Is(err4126, ErrRefreshTokenExpired) {
		t.Fatalf("Expected ErrRefreshTokenExpired, got %v", err4126)
	}
}

func TestPikPak_MockCreateOfflineTask(t *testing.T) {
	// Setup mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/drive/v1/files" {
			var req OfflineCreateRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			if req.URL.URL == "magnet:?xt=urn:btih:quota_exhausted_hash" {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"error":"task_daily_create_limit","error_description":"daily limit reached"}`))
				return
			}

			resp := OfflineDownloadResponse{
				Task: OfflineTask{
					ID:       "task_123456",
					Name:     "Test Download",
					Phase:    "PHASE_TYPE_RUNNING",
					Progress: 10,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client, err := NewClient(ClientOptions{
		AccountID:    1,
		AccountName:  "TestAccount",
		AccessToken:  "mock_token",
		RefreshToken: "mock_refresh",
	})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	// Test success using custom request
	ctx := context.Background()
	var respSuccess OfflineDownloadResponse
	err = client.DoRequest(ctx, http.MethodPost, server.URL+"/drive/v1/files", OfflineCreateRequest{
		URL: struct {
			URL string `json:"url"`
		}{URL: "magnet:?xt=urn:btih:success_hash"},
	}, &respSuccess)
	if err != nil {
		t.Fatalf("Expected success, got: %v", err)
	}
	if respSuccess.Task.ID != "task_123456" {
		t.Fatalf("Expected task_123456, got %s", respSuccess.Task.ID)
	}

	// Test Quota Exceeded error
	var respFail OfflineDownloadResponse
	err = client.DoRequest(ctx, http.MethodPost, server.URL+"/drive/v1/files", OfflineCreateRequest{
		URL: struct {
			URL string `json:"url"`
		}{URL: "magnet:?xt=urn:btih:quota_exhausted_hash"},
	}, &respFail)
	if !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("Expected ErrQuotaExceeded, got: %v", err)
	}
}

func TestPikPak_MockMediaStreamRange(t *testing.T) {
	data := []byte("0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rangeHdr := r.Header.Get("Range")
		if rangeHdr == "bytes=0-9" {
			w.Header().Set("Content-Range", "bytes 0-9/36")
			w.Header().Set("Accept-Ranges", "bytes")
			w.WriteHeader(http.StatusPartialContent)
			w.Write(data[0:10])
			return
		}
		w.Header().Set("Accept-Ranges", "bytes")
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}))
	defer server.Close()

	client, err := NewClient(ClientOptions{
		AccountID: 1,
	})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	resp, err := client.OpenMediaStream(context.Background(), server.URL, "bytes=0-9")
	if err != nil {
		t.Fatalf("OpenMediaStream failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent {
		t.Fatalf("Expected 206 Partial Content, got %d", resp.StatusCode)
	}
	if resp.Header.Get("Content-Range") != "bytes 0-9/36" {
		t.Fatalf("Unexpected Content-Range: %s", resp.Header.Get("Content-Range"))
	}
}
