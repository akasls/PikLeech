package pikpak

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// GenerateDeviceID generates MD5 device ID from seed
func GenerateDeviceID(seed string) string {
	h := md5.Sum([]byte(seed))
	return hex.EncodeToString(h[:])
}

func (c *Client) generateDeviceSign(deviceID string) string {
	signatureBase := fmt.Sprintf("%s%s1appkey", deviceID, AndroidPackageName)
	sha1H := sha1.Sum([]byte(signatureBase))
	sha1Str := hex.EncodeToString(sha1H[:])
	md5H := md5.Sum([]byte(sha1Str))
	md5Str := hex.EncodeToString(md5H[:])
	return fmt.Sprintf("div101.%s%s", deviceID, md5Str)
}

func (c *Client) getUserAgent() string {
	deviceSign := c.generateDeviceSign(c.deviceID)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("ANDROID-pikpak/%s ", AndroidClientVersion))
	sb.WriteString("protocolVersion/200 accesstype/ ")
	sb.WriteString(fmt.Sprintf("clientid/%s clientversion/%s action_type/ networktype/WIFI sessionid/ ", AndroidClientID, AndroidClientVersion))
	sb.WriteString(fmt.Sprintf("deviceid/%s providername/NONE devicesign/%s ", c.deviceID, deviceSign))
	sb.WriteString(fmt.Sprintf("refresh_token/ sdkversion/%s datetime/%d usrno/%s appname/android-pikpak ", AndroidSdkVersion, time.Now().UnixMilli(), c.userID))
	sb.WriteString("devicename/Xiaomi_M2004j7ac osversion/13 platformversion/10 accessmode/ devicemodel/M2004J7AC")
	return sb.String()
}

func (c *Client) getCaptchaSign(timestamp string) string {
	str := fmt.Sprint(AndroidClientID, AndroidClientVersion, AndroidPackageName, c.deviceID, timestamp)
	for _, algorithm := range AndroidAlgorithms {
		h := md5.Sum([]byte(str + algorithm))
		str = hex.EncodeToString(h[:])
	}
	return "1." + str
}

// RefreshCaptchaToken requests a fresh captcha token for an action
func (c *Client) RefreshCaptchaToken(ctx context.Context, action string) error {
	timestamp := fmt.Sprint(time.Now().UnixMilli())
	captchaSign := c.getCaptchaSign(timestamp)

	metas := map[string]string{
		"client_version": AndroidClientVersion,
		"package_name":   AndroidPackageName,
		"timestamp":      timestamp,
		"captcha_sign":   captchaSign,
	}

	if c.userID != "" {
		metas["user_id"] = c.userID
	} else if ok, _ := regexp.MatchString(`\w+([-+.]\w+)*@\w+([-.]\w+)*\.\w+([-.]\w+)*`, c.username); ok {
		metas["email"] = c.username
	} else {
		metas["username"] = c.username
	}

	reqBody := CaptchaTokenRequest{
		Action:       action,
		CaptchaToken: c.captchaToken,
		ClientID:     AndroidClientID,
		DeviceID:     c.deviceID,
		Meta:         metas,
		RedirectUri:  "xlaccsdk01://xbase.cloud/callback?state=harbor",
	}

	rawURL := fmt.Sprintf("%s/v1/shield/captcha/init?client_id=%s", ApiUserBaseURL, AndroidClientID)
	var resp CaptchaTokenResponse

	jsonBytes, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, bytes.NewReader(jsonBytes))
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", c.getUserAgent())
	req.Header.Set("X-Device-ID", c.deviceID)
	req.Header.Set("Content-Type", "application/json")

	httpResp, err := c.httpClient.Do(req)
	if err != nil {
		return ClassifyError(err, 0, nil)
	}
	defer httpResp.Body.Close()

	respBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return err
	}

	if httpResp.StatusCode >= 400 {
		return ClassifyError(nil, httpResp.StatusCode, respBytes)
	}

	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return err
	}

	if resp.URL != "" {
		return fmt.Errorf("manual captcha verification required: %s", resp.URL)
	}

	c.captchaToken = resp.CaptchaToken
	return nil
}

// Login authenticates with username and password
func (c *Client) Login(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.username == "" || c.password == "" {
		return errors.New("username or password cannot be empty")
	}

	signinURL := fmt.Sprintf("%s/v1/auth/signin?client_id=%s", ApiUserBaseURL, AndroidClientID)
	if c.captchaToken == "" {
		_ = c.RefreshCaptchaToken(ctx, "POST:/v1/auth/signin")
	}

	reqBody := SigninRequest{
		ClientID:     AndroidClientID,
		ClientSecret: AndroidClientSecret,
		Username:     c.username,
		Password:     c.password,
		CaptchaToken: c.captchaToken,
	}

	jsonBytes, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, signinURL, bytes.NewReader(jsonBytes))
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", c.getUserAgent())
	req.Header.Set("X-Device-ID", c.deviceID)
	if c.captchaToken != "" {
		req.Header.Set("X-Captcha-Token", c.captchaToken)
	}
	req.Header.Set("Content-Type", "application/json")

	httpResp, err := c.httpClient.Do(req)
	if err != nil {
		return ClassifyError(err, 0, nil)
	}
	defer httpResp.Body.Close()

	respBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return err
	}

	if httpResp.StatusCode >= 400 {
		return ClassifyError(nil, httpResp.StatusCode, respBytes)
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(respBytes, &tokenResp); err != nil {
		return err
	}

	c.accessToken = tokenResp.AccessToken
	c.refreshToken = tokenResp.RefreshToken
	c.userID = tokenResp.UserID

	if c.onTokenUpdated != nil {
		c.onTokenUpdated(c.accessToken, c.refreshToken)
	}

	return nil
}

// RefreshToken exchanges refreshToken for a new accessToken
func (c *Client) RefreshToken(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.refreshToken == "" {
		if c.username != "" && c.password != "" {
			// Try login if no refresh token
			return c.loginLocked(ctx)
		}
		return ErrRefreshTokenExpired
	}

	refreshURL := fmt.Sprintf("%s/v1/auth/token?client_id=%s", ApiUserBaseURL, AndroidClientID)
	reqBody := TokenRequest{
		ClientID:     AndroidClientID,
		ClientSecret: AndroidClientSecret,
		GrantType:    "refresh_token",
		RefreshToken: c.refreshToken,
	}

	jsonBytes, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, refreshURL, bytes.NewReader(jsonBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Device-ID", c.deviceID)

	httpResp, err := c.httpClient.Do(req)
	if err != nil {
		return ClassifyError(err, 0, nil)
	}
	defer httpResp.Body.Close()

	respBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return err
	}

	if httpResp.StatusCode >= 400 {
		classified := ClassifyError(nil, httpResp.StatusCode, respBytes)
		if errors.Is(classified, ErrRefreshTokenExpired) && c.username != "" && c.password != "" {
			// Refresh token expired, fallback to re-login with username and password
			return c.loginLocked(ctx)
		}
		return classified
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(respBytes, &tokenResp); err != nil {
		return err
	}

	c.accessToken = tokenResp.AccessToken
	if tokenResp.RefreshToken != "" {
		c.refreshToken = tokenResp.RefreshToken
	}
	if tokenResp.UserID != "" {
		c.userID = tokenResp.UserID
	}

	if c.onTokenUpdated != nil {
		c.onTokenUpdated(c.accessToken, c.refreshToken)
	}

	return nil
}

func (c *Client) loginLocked(ctx context.Context) error {
	if c.username == "" || c.password == "" {
		return errors.New("username or password is required")
	}

	signinURL := fmt.Sprintf("%s/v1/auth/signin?client_id=%s", ApiUserBaseURL, AndroidClientID)
	reqBody := SigninRequest{
		ClientID:     AndroidClientID,
		ClientSecret: AndroidClientSecret,
		Username:     c.username,
		Password:     c.password,
		CaptchaToken: c.captchaToken,
	}

	jsonBytes, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, signinURL, bytes.NewReader(jsonBytes))
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", c.getUserAgent())
	req.Header.Set("X-Device-ID", c.deviceID)
	req.Header.Set("Content-Type", "application/json")

	httpResp, err := c.httpClient.Do(req)
	if err != nil {
		return ClassifyError(err, 0, nil)
	}
	defer httpResp.Body.Close()

	respBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return err
	}

	if httpResp.StatusCode >= 400 {
		return ClassifyError(nil, httpResp.StatusCode, respBytes)
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(respBytes, &tokenResp); err != nil {
		return err
	}

	c.accessToken = tokenResp.AccessToken
	c.refreshToken = tokenResp.RefreshToken
	c.userID = tokenResp.UserID

	if c.onTokenUpdated != nil {
		c.onTokenUpdated(c.accessToken, c.refreshToken)
	}

	return nil
}

// TestProxy tests network connectivity, latency, and egress IP using client's proxy.
func (c *Client) TestProxy(ctx context.Context) (ip string, latencyMs int64, err error) {
	start := time.Now()
	testURL := "https://cloudflare.com/cdn-cgi/trace"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, testURL, nil)
	if err != nil {
		return "", 0, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", 0, ClassifyError(err, 0, nil)
	}
	defer resp.Body.Close()

	latency := time.Since(start).Milliseconds()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", latency, err
	}

	// Parse ip= from trace
	for _, line := range strings.Split(string(body), "\n") {
		if strings.HasPrefix(line, "ip=") {
			ip = strings.TrimPrefix(line, "ip=")
			break
		}
	}

	if ip == "" {
		ip = "Connected"
	}

	return ip, latency, nil
}
