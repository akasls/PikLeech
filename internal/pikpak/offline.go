package pikpak

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// CreateOfflineTask submits a URL (magnet, http, etc.) for cloud offline download
func (c *Client) CreateOfflineTask(ctx context.Context, downloadURL, name, parentID string) (*OfflineTask, error) {
	reqURL := fmt.Sprintf("%s/drive/v1/files", ApiDriveBaseURL)
	reqBody := OfflineCreateRequest{
		Kind:       "drive#file",
		Name:       name,
		UploadType: "UPLOAD_TYPE_URL",
		ParentID:   parentID,
		FolderType: "",
	}
	reqBody.URL.URL = downloadURL

	var resp OfflineDownloadResponse
	err := c.DoRequest(ctx, http.MethodPost, reqURL, reqBody, &resp)
	if err != nil {
		return nil, err
	}

	return &resp.Task, nil
}

// ListOfflineTasks fetches offline download tasks
func (c *Client) ListOfflineTasks(ctx context.Context, pageToken string) (*OfflineListResponse, error) {
	params := url.Values{}
	params.Set("type", "offline")
	params.Set("thumbnail_size", "SIZE_SMALL")
	params.Set("limit", "100")
	params.Set("filters", `{"phase":{"in":"PHASE_TYPE_RUNNING,PHASE_TYPE_ERROR,PHASE_TYPE_COMPLETE,PHASE_TYPE_PENDING"}}`)
	if pageToken != "" {
		params.Set("page_token", pageToken)
	}

	reqURL := fmt.Sprintf("%s/drive/v1/tasks?%s", ApiDriveBaseURL, params.Encode())
	var resp OfflineListResponse

	err := c.DoRequest(ctx, http.MethodGet, reqURL, nil, &resp)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

// DeleteOfflineTasks deletes offline download task records
func (c *Client) DeleteOfflineTasks(ctx context.Context, taskIDs []string, deleteFiles bool) error {
	if len(taskIDs) == 0 {
		return nil
	}

	params := url.Values{}
	params.Set("task_ids", strings.Join(taskIDs, ","))
	params.Set("delete_files", strconv.FormatBool(deleteFiles))

	reqURL := fmt.Sprintf("%s/drive/v1/tasks?%s", ApiDriveBaseURL, params.Encode())
	return c.DoRequest(ctx, http.MethodDelete, reqURL, nil, nil)
}
