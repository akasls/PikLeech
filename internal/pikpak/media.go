package pikpak

import (
	"context"
	"errors"
	"fmt"
	"net/http"
)

// GetMediaURL retrieves playable URL for a file
func (c *Client) GetMediaURL(ctx context.Context, fileID string) (string, error) {
	file, err := c.GetFile(ctx, fileID)
	if err != nil {
		return "", err
	}

	// First look for origin or default media link
	for _, m := range file.Medias {
		if m.IsOrigin && m.Link.URL != "" {
			return m.Link.URL, nil
		}
	}

	for _, m := range file.Medias {
		if m.Link.URL != "" {
			return m.Link.URL, nil
		}
	}

	if file.WebContentLink != "" {
		return file.WebContentLink, nil
	}

	return "", errors.New("no playable or downloadable media link available")
}

// OpenMediaStream opens an HTTP stream to the media URL with Range support through client's proxy.
func (c *Client) OpenMediaStream(ctx context.Context, mediaURL string, rangeHeader string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, mediaURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", c.getUserAgent())
	if rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, ClassifyError(err, 0, nil)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		resp.Body.Close()
		return nil, fmt.Errorf("upstream media returned HTTP %d", resp.StatusCode)
	}

	return resp, nil
}
