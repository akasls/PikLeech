package pikpak

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

var (
	ErrQuotaExceeded       = errors.New("pikpak: offline download quota exhausted (task_daily_create_limit)")
	ErrAuthFailed          = errors.New("pikpak: authentication failed / invalid credentials")
	ErrRefreshTokenExpired = errors.New("pikpak: refresh token expired (code 4126)")
	ErrRateLimited         = errors.New("pikpak: rate limited / cooldown active (429 or code 10)")
	ErrProxyFailed         = errors.New("pikpak: proxy connection failed")
	ErrNetwork             = errors.New("pikpak: network timeout or temporary error")
	ErrNotFound            = errors.New("pikpak: resource not found (404)")
	ErrNeedCaptcha         = errors.New("pikpak: captcha verification required (code 9)")
	ErrApiError            = errors.New("pikpak: api error")
)

type DetailedError struct {
	Type        string // QUOTA_EXHAUSTED, AUTH_FAILED, PROXY_FAILED, RATE_LIMITED, NETWORK_ERROR, etc.
	Code        int64
	Message     string
	Description string
	RawErr      error
}

func (e *DetailedError) Error() string {
	if e.Description != "" {
		return fmt.Sprintf("[%s] code=%d: %s (%s)", e.Type, e.Code, e.Message, e.Description)
	}
	if e.Message != "" {
		return fmt.Sprintf("[%s] code=%d: %s", e.Type, e.Code, e.Message)
	}
	if e.RawErr != nil {
		return fmt.Sprintf("[%s] %v", e.Type, e.RawErr)
	}
	return fmt.Sprintf("[%s]", e.Type)
}

func (e *DetailedError) Unwrap() error {
	switch e.Type {
	case "QUOTA_EXHAUSTED":
		return ErrQuotaExceeded
	case "AUTH_FAILED":
		return ErrAuthFailed
	case "REFRESH_TOKEN_EXPIRED":
		return ErrRefreshTokenExpired
	case "RATE_LIMITED":
		return ErrRateLimited
	case "PROXY_FAILED":
		return ErrProxyFailed
	case "NETWORK_ERROR":
		return ErrNetwork
	case "NOT_FOUND":
		return ErrNotFound
	case "NEED_CAPTCHA":
		return ErrNeedCaptcha
	default:
		return ErrApiError
	}
}

// ClassifyError inspects HTTP status code, response body, and raw network errors to classify.
func ClassifyError(rawErr error, statusCode int, respBody []byte) error {
	// 1. Network / Proxy inspection
	if rawErr != nil {
		errStr := strings.ToLower(rawErr.Error())
		if strings.Contains(errStr, "proxy") || strings.Contains(errStr, "socks") || strings.Contains(errStr, "proxyconnect") {
			return &DetailedError{
				Type:   "PROXY_FAILED",
				RawErr: rawErr,
			}
		}

		var netErr net.Error
		if errors.As(rawErr, &netErr) {
			if netErr.Timeout() {
				return &DetailedError{
					Type:   "NETWORK_ERROR",
					RawErr: rawErr,
				}
			}
		}

		var urlErr *url.Error
		if errors.As(rawErr, &urlErr) {
			if strings.Contains(strings.ToLower(urlErr.Error()), "proxy") {
				return &DetailedError{
					Type:   "PROXY_FAILED",
					RawErr: rawErr,
				}
			}
			return &DetailedError{
				Type:   "NETWORK_ERROR",
				RawErr: rawErr,
			}
		}

		return &DetailedError{
			Type:   "NETWORK_ERROR",
			RawErr: rawErr,
		}
	}

	// 2. HTTP Status code checks
	if statusCode == 429 {
		return &DetailedError{
			Type:        "RATE_LIMITED",
			Code:        429,
			Message:     "Too Many Requests",
			Description: string(respBody),
		}
	}

	if statusCode == 404 {
		return &DetailedError{
			Type:    "NOT_FOUND",
			Code:    404,
			Message: "Not Found",
		}
	}

	// 3. Inspect JSON body from PikPak API
	var apiErr APIErrorResp
	if len(respBody) > 0 {
		_ = json.Unmarshal(respBody, &apiErr)
	}

	errCode := apiErr.ErrorCode
	errMsg := strings.ToLower(apiErr.Error)
	errDesc := strings.ToLower(apiErr.ErrorDescription)
	rawStr := strings.ToLower(string(respBody))

	// Check for quota exhausted
	if errMsg == "task_daily_create_limit" ||
		strings.Contains(errDesc, "task_daily_create_limit") ||
		strings.Contains(errDesc, "daily task limit reached") ||
		strings.Contains(errDesc, "daily limit reached") ||
		strings.Contains(errDesc, "quota exceeded") ||
		strings.Contains(rawStr, "task_daily_create_limit") ||
		strings.Contains(rawStr, "daily task limit") ||
		errCode == 10013 || errCode == 10022 {
		return &DetailedError{
			Type:        "QUOTA_EXHAUSTED",
			Code:        errCode,
			Message:     apiErr.Error,
			Description: apiErr.ErrorDescription,
		}
	}

	// Check for rate limit / frequent operations
	if errCode == 10 || strings.Contains(errDesc, "frequent") || strings.Contains(errMsg, "frequent") {
		return &DetailedError{
			Type:        "RATE_LIMITED",
			Code:        errCode,
			Message:     apiErr.Error,
			Description: apiErr.ErrorDescription,
		}
	}

	// Check for captcha
	if errCode == 9 || strings.Contains(errMsg, "captcha") || strings.Contains(errDesc, "captcha") {
		return &DetailedError{
			Type:        "NEED_CAPTCHA",
			Code:        errCode,
			Message:     apiErr.Error,
			Description: apiErr.ErrorDescription,
		}
	}

	// Check for refresh token expired
	if errCode == 4126 || strings.Contains(errMsg, "invalid_refresh_token") {
		return &DetailedError{
			Type:        "REFRESH_TOKEN_EXPIRED",
			Code:        errCode,
			Message:     apiErr.Error,
			Description: apiErr.ErrorDescription,
		}
	}

	// Check for access token expired / unauthorized
	if statusCode == 401 || errCode == 16 || errCode == 4121 || errCode == 4122 || strings.Contains(errMsg, "invalid_token") || strings.Contains(errMsg, "unauthorized") {
		return &DetailedError{
			Type:        "AUTH_FAILED",
			Code:        errCode,
			Message:     apiErr.Error,
			Description: apiErr.ErrorDescription,
		}
	}

	return &DetailedError{
		Type:        "API_ERROR",
		Code:        errCode,
		Message:     apiErr.Error,
		Description: apiErr.ErrorDescription,
	}
}
