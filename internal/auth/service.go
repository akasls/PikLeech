package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

const (
	SessionCookieName = "pikpak_session"
	TokenTTL          = 7 * 24 * time.Hour
)

type SessionClaims struct {
	UserID    int64 `json:"uid"`
	Username  string `json:"usr"`
	ExpiresAt int64  `json:"exp"`
}

type Service struct {
	db        *sql.DB
	appSecret string
}

func NewService(db *sql.DB, appSecret string) *Service {
	return &Service{
		db:        db,
		appSecret: appSecret,
	}
}

// GenerateSessionToken creates a signed HMAC-SHA256 session token
func (s *Service) GenerateSessionToken(userID int64, username string) (string, error) {
	exp := time.Now().Add(TokenTTL).Unix()
	claims := SessionClaims{
		UserID:    userID,
		Username:  username,
		ExpiresAt: exp,
	}

	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadBytes)

	mac := hmac.New(sha256.New, []byte(s.appSecret))
	mac.Write([]byte(payloadB64))
	sigB64 := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return fmt.Sprintf("%s.%s", payloadB64, sigB64), nil
}

// ValidateSessionToken verifies HMAC signature and expiration
func (s *Service) ValidateSessionToken(tokenStr string) (*SessionClaims, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 2 {
		return nil, errors.New("invalid session token format")
	}

	payloadB64, sigB64 := parts[0], parts[1]

	// Verify signature
	mac := hmac.New(sha256.New, []byte(s.appSecret))
	mac.Write([]byte(payloadB64))
	expectedSig := mac.Sum(nil)

	actualSig, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil || !hmac.Equal(actualSig, expectedSig) {
		return nil, errors.New("invalid session signature")
	}

	// Parse claims
	payloadBytes, err := base64.RawURLEncoding.DecodeString(payloadB64)
	if err != nil {
		return nil, err
	}

	var claims SessionClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, err
	}

	if time.Now().Unix() > claims.ExpiresAt {
		return nil, errors.New("session expired")
	}

	return &claims, nil
}

// Login verifies credentials and returns session token
func (s *Service) Login(username, password string) (string, error) {
	var id int64
	var hash string
	err := s.db.QueryRow("SELECT id, password_hash FROM users WHERE username = ?", username).Scan(&id, &hash)
	if err != nil {
		return "", errors.New("invalid username or password")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return "", errors.New("invalid username or password")
	}

	return s.GenerateSessionToken(id, username)
}

// ChangePassword updates the user's password with a new bcrypt hash
func (s *Service) ChangePassword(userID int64, oldPassword, newPassword string) error {
	if len(newPassword) < 6 {
		return errors.New("new password must be at least 6 characters")
	}

	var hash string
	err := s.db.QueryRow("SELECT password_hash FROM users WHERE id = ?", userID).Scan(&hash)
	if err != nil {
		return err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(oldPassword)); err != nil {
		return errors.New("current password is incorrect")
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	_, err = s.db.Exec("UPDATE users SET password_hash = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", string(newHash), userID)
	return err
}

// AuthMiddleware protects admin routes using HttpOnly Cookie or Bearer header
func (s *Service) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var token string

		// Check cookie first
		if cookie, err := c.Cookie(SessionCookieName); err == nil && cookie != "" {
			token = cookie
		}

		// Or check Authorization header
		if token == "" {
			authHeader := c.GetHeader("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				token = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		// Or check query parameter "token" (for media streaming / external video players)
		if token == "" {
			token = c.Query("token")
		}

		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized: login required"})
			return
		}

		claims, err := s.ValidateSessionToken(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized: session invalid or expired"})
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Next()
	}
}

func (s *Service) HandleLogin(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	token, err := s.Login(req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	// Set HttpOnly, SameSite=Lax cookie
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(SessionCookieName, token, int(TokenTTL.Seconds()), "/", "", false, true)

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"token":    token,
		"username": req.Username,
	})
}

func (s *Service) HandleLogout(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(SessionCookieName, "", -1, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Service) HandleMe(c *gin.Context) {
	userID, _ := c.Get("user_id")
	username, _ := c.Get("username")

	c.JSON(http.StatusOK, gin.H{
		"user_id":  userID,
		"username": username,
	})
}

func (s *Service) HandleChangePassword(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request payload"})
		return
	}

	var uid int64
	switch v := userIDVal.(type) {
	case int64:
		uid = v
	case string:
		uid, _ = strconv.ParseInt(v, 10, 64)
	}

	if err := s.ChangePassword(uid, req.OldPassword, req.NewPassword); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "password changed successfully"})
}
