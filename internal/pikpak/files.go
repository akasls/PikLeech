package pikpak

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// ListFiles fetches file and folder list under parentID
func (c *Client) ListFiles(ctx context.Context, parentID string, pageToken string, limit int) (*FileListResponse, error) {
	if limit <= 0 {
		limit = 100
	}

	params := url.Values{}
	params.Set("parent_id", parentID)
	params.Set("thumbnail_size", "SIZE_LARGE")
	params.Set("with_audit", "true")
	params.Set("limit", strconv.Itoa(limit))
	params.Set("filters", `{"phase":{"eq":"PHASE_TYPE_COMPLETE"},"trashed":{"eq":false}}`)
	if pageToken != "" {
		params.Set("page_token", pageToken)
	}

	reqURL := fmt.Sprintf("%s/drive/v1/files?%s", ApiDriveBaseURL, params.Encode())
	var resp FileListResponse

	err := c.DoRequest(ctx, http.MethodGet, reqURL, nil, &resp)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

// GetFile gets file details including medias and web_content_link
func (c *Client) GetFile(ctx context.Context, fileID string) (*FileItem, error) {
	params := url.Values{}
	params.Set("_magic", "2021")
	params.Set("usage", "CACHE")
	params.Set("thumbnail_size", "SIZE_LARGE")

	reqURL := fmt.Sprintf("%s/drive/v1/files/%s?%s", ApiDriveBaseURL, fileID, params.Encode())
	var resp FileItem

	err := c.DoRequest(ctx, http.MethodGet, reqURL, nil, &resp)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

// TrashFiles moves files into trash
func (c *Client) TrashFiles(ctx context.Context, fileIDs []string) error {
	if len(fileIDs) == 0 {
		return nil
	}

	reqURL := fmt.Sprintf("%s/drive/v1/files:batchTrash", ApiDriveBaseURL)
	reqBody := map[string]interface{}{
		"ids": fileIDs,
	}

	return c.DoRequest(ctx, http.MethodPost, reqURL, reqBody, nil)
}

// DeleteFiles permanently deletes files
func (c *Client) DeleteFiles(ctx context.Context, fileIDs []string) error {
	if len(fileIDs) == 0 {
		return nil
	}

	reqURL := fmt.Sprintf("%s/drive/v1/files:batchDelete", ApiDriveBaseURL)
	reqBody := map[string]interface{}{
		"ids": fileIDs,
	}

	return c.DoRequest(ctx, http.MethodPost, reqURL, reqBody, nil)
}

// MakeDir creates a folder under parentID
func (c *Client) MakeDir(ctx context.Context, parentID, name string) (*FileItem, error) {
	reqURL := fmt.Sprintf("%s/drive/v1/files", ApiDriveBaseURL)
	reqBody := map[string]interface{}{
		"kind":      "drive#folder",
		"parent_id": parentID,
		"name":      name,
	}

	var resp FileItem
	err := c.DoRequest(ctx, http.MethodPost, reqURL, reqBody, &resp)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

// GetStorageAbout gets disk usage and quota details
func (c *Client) GetStorageAbout(ctx context.Context) (*AboutResponse, error) {
	reqURL := fmt.Sprintf("%s/drive/v1/about", ApiDriveBaseURL)
	var resp AboutResponse

	err := c.DoRequest(ctx, http.MethodGet, reqURL, nil, &resp)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}
