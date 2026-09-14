package pikpak

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/proxy"
)

type ClientOptions struct {
	AccountID       int64
	AccountName     string
	Username        string
	Password        string
	AccessToken     string
	RefreshToken    string
	DeviceID        string
	UserID          string
	CaptchaToken    string
	ProxyURL        string
	OnTokenUpdated  func(accessToken, refreshToken string)
}

type Client struct {
	accountID      int64
	accountName    string
	username       string
	password       string
	accessToken    string
	refreshToken   string
	deviceID       string
	userID         string
	captchaToken   string
	proxyURL       string
	httpClient     *http.Client
	streamingClient *http.Client
	transport      *http.Transport
	onTokenUpdated func(accessToken, refreshToken string)

	mu            sync.Mutex
	cooldownUntil time.Time
}

// NewClient creates an independent PikPak Client with its own HTTP Transport and proxy.
func NewClient(opts ClientOptions) (*Client, error) {
	c := &Client{
		accountID:      opts.AccountID,
		accountName:    opts.AccountName,
		username:       opts.Username,
		password:       opts.Password,
		accessToken:    opts.AccessToken,
		refreshToken:   opts.RefreshToken,
		deviceID:       opts.DeviceID,
		userID:         opts.UserID,
		captchaToken:   opts.CaptchaToken,
		proxyURL:       opts.ProxyURL,
		onTokenUpdated: opts.OnTokenUpdated,
	}

	if c.deviceID == "" {
		c.deviceID = GenerateDeviceID(c.username + c.password)
	}

	httpClient, transport, err := createHTTPClient(opts.ProxyURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create http client with proxy: %w", err)
	}

	c.httpClient = httpClient
	c.transport = transport
	c.streamingClient = &http.Client{
		Transport: transport,
		Timeout:   0, // No client timeout for long media streaming and large downloads
	}
	return c, nil
}

// createHTTPClient builds an isolated http.Client with dedicated Transport for the given proxy.
func createHTTPClient(proxyStr string) (*http.Client, *http.Transport, error) {
	transport := &http.Transport{
		Proxy: nil,
		DialContext: (&net.Dialer{
			Timeout:   15 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   20,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,
		},
	}

	proxyStr = strings.TrimSpace(proxyStr)
	if proxyStr != "" {
		parsedURL, err := url.Parse(proxyStr)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid proxy URL: %w", err)
		}

		scheme := strings.ToLower(parsedURL.Scheme)
		switch scheme {
		case "http", "https":
			transport.Proxy = http.ProxyURL(parsedURL)
		case "socks5", "socks5h":
			// SOCKS5 / SOCKS5H
			var auth *proxy.Auth
			if parsedURL.User != nil {
				auth = &proxy.Auth{
					User: parsedURL.User.Username(),
				}
				if pwd, ok := parsedURL.User.Password(); ok {
					auth.Password = pwd
				}
			}

			dialer, err := proxy.SOCKS5("tcp", parsedURL.Host, auth, proxy.Direct)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to initialize socks5 dialer: %w", err)
			}

			if contextDialer, ok := dialer.(proxy.ContextDialer); ok {
				transport.DialContext = contextDialer.DialContext
			} else {
				transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
					return dialer.Dial(network, addr)
				}
			}
		default:
			return nil, nil, fmt.Errorf("unsupported proxy protocol: %s", scheme)
		}
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}

	return client, transport, nil
}

// GetAccountID returns the account ID
func (c *Client) GetAccountID() int64 {
	return c.accountID
}

// GetAccountName returns the account remark name
func (c *Client) GetAccountName() string {
	return c.accountName
}

// GetProxyURL returns configured proxy
func (c *Client) GetProxyURL() string {
	return c.proxyURL
}

// IsCooldown returns true if client is in 429 cooldown
func (c *Client) IsCooldown() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return time.Now().Before(c.cooldownUntil)
}

// SetCooldown sets cooldown duration
func (c *Client) SetCooldown(duration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cooldownUntil = time.Now().Add(duration)
}

// DoRequest sends an authenticated HTTP request, handles automatic token refresh and error parsing.
func (c *Client) DoRequest(ctx context.Context, method, reqURL string, reqBody interface{}, respResult interface{}) error {
	return c.doRequest(ctx, method, reqURL, reqBody, respResult, false)
}

func (c *Client) doRequest(ctx context.Context, method, reqURL string, reqBody interface{}, respResult interface{}, retried bool) error {
	if c.IsCooldown() {
		return ErrRateLimited
	}

	var bodyReader io.Reader
	if reqBody != nil {
		jsonBytes, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL, bodyReader)
	if err != nil {
		return err
	}

	// Attach standard headers
	req.Header.Set("User-Agent", c.getUserAgent())
	req.Header.Set("X-Device-ID", c.deviceID)
	if c.captchaToken != "" {
		req.Header.Set("X-Captcha-Token", c.captchaToken)
	}
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	c.mu.Lock()
	token := c.accessToken
	c.mu.Unlock()

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return ClassifyError(err, 0, nil)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return ClassifyError(err, resp.StatusCode, nil)
	}

	// Handle 429 Too Many Requests
	if resp.StatusCode == 429 {
		c.SetCooldown(60 * time.Second)
		return ClassifyError(nil, 429, respBytes)
	}

	// Handle Token Expired (401, error_code 16, 4121, 4122)
	classifiedErr := ClassifyError(nil, resp.StatusCode, respBytes)
	if !retried && errors.Is(classifiedErr, ErrAuthFailed) {
		// Attempt token refresh and retry at most once to prevent infinite recursive loop
		if refreshErr := c.RefreshToken(ctx); refreshErr == nil {
			return c.doRequest(ctx, method, reqURL, reqBody, respResult, true)
		}
		return classifiedErr
	}

	// If HTTP status code is an error, classify and return
	if resp.StatusCode >= 400 {
		return ClassifyError(nil, resp.StatusCode, respBytes)
	}

	if len(respBytes) > 0 && (strings.Contains(string(respBytes), `"error_code"`) || strings.Contains(string(respBytes), `"error"`)) {
		var apiErr APIErrorResp
		if err := json.Unmarshal(respBytes, &apiErr); err == nil && (apiErr.ErrorCode != 0 || apiErr.Error != "") {
			return ClassifyError(nil, resp.StatusCode, respBytes)
		}
	}

	if respResult != nil && len(respBytes) > 0 {
		if err := json.Unmarshal(respBytes, respResult); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}

func (c *Client) GetHTTPClient() *http.Client {
	return c.httpClient
}

func (c *Client) GetStreamingClient() *http.Client {
	return c.streamingClient
}

func (c *Client) GetTransport() *http.Transport {
	return c.transport
}
