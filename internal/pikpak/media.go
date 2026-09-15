package pikpak

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

type PlayableMedia struct {
	MediaID        string `json:"media_id"`
	MediaName      string `json:"media_name"`
	ResolutionName string `json:"resolution_name"`
	VideoWidth     int    `json:"width"`
	VideoHeight    int    `json:"height"`
	VideoCodec     string `json:"video_codec"`
	AudioCodec     string `json:"audio_codec"`
	IsOrigin       bool   `json:"is_origin"`
	URL            string `json:"url"`
}

type MediaPlaybackResult struct {
	DirectURL string          `json:"direct_url"`
	OriginURL string          `json:"origin_url"`
	IsMKV     bool            `json:"is_mkv"`
	Medias    []PlayableMedia `json:"medias"`
}

// GetPlaybackMediaDetails retrieves all available streams, origin stream, and the best browser-playable stream.
func (c *Client) GetPlaybackMediaDetails(ctx context.Context, fileID string, fileName string) (*MediaPlaybackResult, error) {
	file, err := c.GetFile(ctx, fileID)
	if err != nil {
		return nil, err
	}

	lowerName := strings.ToLower(fileName)
	isMKV := strings.HasSuffix(lowerName, ".mkv")

	var list []PlayableMedia
	var originURL string
	var bestTranscodeURL string

	for _, m := range file.Medias {
		if m.Link.URL == "" {
			continue
		}
		item := PlayableMedia{
			MediaID:        m.MediaID,
			MediaName:      m.MediaName,
			ResolutionName: m.ResolutionName,
			VideoWidth:     m.Video.Width,
			VideoHeight:    m.Video.Height,
			VideoCodec:     m.Video.VideoCodec,
			AudioCodec:     m.Video.AudioCodec,
			IsOrigin:       m.IsOrigin,
			URL:            m.Link.URL,
		}
		if item.ResolutionName == "" {
			if item.IsOrigin {
				item.ResolutionName = "原画"
			} else if item.MediaName != "" {
				item.ResolutionName = item.MediaName
			} else {
				item.ResolutionName = "转码流"
			}
		}
		list = append(list, item)

		if m.IsOrigin && originURL == "" {
			originURL = m.Link.URL
		}
		if !m.IsOrigin && bestTranscodeURL == "" {
			bestTranscodeURL = m.Link.URL
		}
	}

	if originURL == "" && file.WebContentLink != "" {
		originURL = file.WebContentLink
		list = append([]PlayableMedia{
			{
				MediaID:        "web_content_link",
				MediaName:      "原画下载直链",
				ResolutionName: "原画",
				IsOrigin:       true,
				URL:            file.WebContentLink,
			},
		}, list...)
	}

	var directURL string
	if isMKV && bestTranscodeURL != "" {
		// Browser native HTML5 <video> struggles with MKV/AC3; default to transcode MP4 stream for web
		directURL = bestTranscodeURL
	} else if originURL != "" {
		directURL = originURL
	} else if bestTranscodeURL != "" {
		directURL = bestTranscodeURL
	} else if file.WebContentLink != "" {
		directURL = file.WebContentLink
	} else {
		return nil, errors.New("no playable or downloadable media link available")
	}

	if originURL == "" {
		originURL = directURL
	}

	return &MediaPlaybackResult{
		DirectURL: directURL,
		OriginURL: originURL,
		IsMKV:     isMKV,
		Medias:    list,
	}, nil
}

// GetMediaURL retrieves playable URL for a file (fallback / legacy)
func (c *Client) GetMediaURL(ctx context.Context, fileID string) (string, error) {
	res, err := c.GetPlaybackMediaDetails(ctx, fileID, "")
	if err != nil {
		return "", err
	}
	return res.DirectURL, nil
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

	client := c.streamingClient
	if client == nil {
		client = c.httpClient
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, ClassifyError(err, 0, nil)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		resp.Body.Close()
		return nil, fmt.Errorf("upstream media returned HTTP %d", resp.StatusCode)
	}

	return resp, nil
}
