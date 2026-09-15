package auth

import (
	"context"
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
	UserID    int64  `json:"uid"`
	Username  string `json:"usr"`
	Role      string `json:"rol"`
	ExpiresAt int64  `json:"exp"`
}

type UserInfo struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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
func (s *Service) GenerateSessionToken(userID int64, username, role string) (string, error) {
	if role == "" {
		role = "user"
	}
	exp := time.Now().Add(TokenTTL).Unix()
	claims := SessionClaims{
		UserID:    userID,
		Username:  username,
		Role:      role,
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

	if claims.Role == "" {
		claims.Role = "user"
	}

	return &claims, nil
}

// Login verifies credentials and returns session token and role
func (s *Service) Login(username, password string) (string, string, error) {
	var id int64
	var hash string
	var role sql.NullString
	err := s.db.QueryRow("SELECT id, password_hash, role FROM users WHERE username = ?", username).Scan(&id, &hash, &role)
	if err != nil {
		return "", "", errors.New("invalid username or password")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return "", "", errors.New("invalid username or password")
	}

	userRole := "user"
	if role.Valid && role.String != "" {
		userRole = role.String
	} else if id == 1 || username == "admin" {
		userRole = "admin"
	}

	token, err := s.GenerateSessionToken(id, username, userRole)
	if err != nil {
		return "", "", err
	}

	return token, userRole, nil
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

// AuthMiddleware protects routes using HttpOnly Cookie or Bearer header
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
		c.Set("role", claims.Role)
		c.Next()
	}
}

// RequireAdmin middleware ensures only users with role "admin" can access the endpoint
func (s *Service) RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		roleVal, exists := c.Get("role")
		if !exists || roleVal != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "需要管理员权限"})
			return
		}
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

	token, role, err := s.Login(req.Username, req.Password)
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
		"role":     role,
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
	role, _ := c.Get("role")

	c.JSON(http.StatusOK, gin.H{
		"user_id":  userID,
		"username": username,
		"role":     role,
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

type UpdateProfileReq struct {
	Username    string `json:"username"`
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

func (s *Service) UpdateProfile(userID int64, req UpdateProfileReq) (newUsername string, newToken string, err error) {
	var currentUsername, currentHash string
	var currentRole sql.NullString
	err = s.db.QueryRow("SELECT username, password_hash, role FROM users WHERE id = ?", userID).Scan(&currentUsername, &currentHash, &currentRole)
	if err != nil {
		return "", "", errors.New("user not found")
	}

	role := "user"
	if currentRole.Valid && currentRole.String != "" {
		role = currentRole.String
	} else if userID == 1 || currentUsername == "admin" {
		role = "admin"
	}

	newUsername = currentUsername
	if strings.TrimSpace(req.Username) != "" {
		newUsername = strings.TrimSpace(req.Username)
	}

	// If password change is requested
	if req.NewPassword != "" {
		if len(req.NewPassword) < 6 {
			return "", "", errors.New("new password must be at least 6 characters")
		}
		if req.OldPassword == "" {
			return "", "", errors.New("old password is required to change password")
		}
		if err := bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(req.OldPassword)); err != nil {
			return "", "", errors.New("current password is incorrect")
		}
		newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
		if err != nil {
			return "", "", err
		}
		currentHash = string(newHash)
	}

	_, err = s.db.Exec("UPDATE users SET username = ?, password_hash = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", newUsername, currentHash, userID)
	if err != nil {
		return "", "", fmt.Errorf("failed to update user profile: %w", err)
	}

	newToken, err = s.GenerateSessionToken(userID, newUsername, role)
	if err != nil {
		return "", "", err
	}

	return newUsername, newToken, nil
}

func (s *Service) HandleUpdateProfile(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req UpdateProfileReq
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

	newUsername, newToken, err := s.UpdateProfile(uid, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update session cookie
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(SessionCookieName, newToken, int(TokenTTL.Seconds()), "/", "", false, true)

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"username": newUsername,
		"token":    newToken,
		"message":  "profile updated successfully",
	})
}

// ---------------- User Management (Admin Only) ----------------

// ListUsers returns all users
func (s *Service) ListUsers(ctx context.Context) ([]UserInfo, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, username, COALESCE(role, 'user'), created_at, updated_at FROM users ORDER BY id ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []UserInfo
	for rows.Next() {
		var u UserInfo
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

// CreateUser adds a new user
func (s *Service) CreateUser(ctx context.Context, username, password, role string) (*UserInfo, error) {
	username = strings.TrimSpace(username)
	if len(username) < 2 {
		return nil, errors.New("用户名长度至少需要 2 个字符")
	}
	if strings.ContainsAny(username, "/\\:*?\"<>| ") {
		return nil, errors.New("用户名不能包含特殊字符或空格")
	}
	if len(password) < 6 {
		return nil, errors.New("密码长度至少需要 6 个字符")
	}
	if role != "admin" && role != "user" {
		role = "user"
	}

	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE username = ?", username).Scan(&count)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("用户名已存在")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, "INSERT INTO users (username, password_hash, role, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
		username, string(hash), role, now, now)
	if err != nil {
		return nil, err
	}

	id, _ := res.LastInsertId()
	return &UserInfo{
		ID:        id,
		Username:  username,
		Role:      role,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// DeleteUser removes a user
func (s *Service) DeleteUser(ctx context.Context, currentUserID, targetUserID int64) error {
	if currentUserID == targetUserID {
		return errors.New("无法删除当前登录的用户账号")
	}

	var targetRole string
	err := s.db.QueryRowContext(ctx, "SELECT COALESCE(role, 'user') FROM users WHERE id = ?", targetUserID).Scan(&targetRole)
	if err != nil {
		return errors.New("用户不存在")
	}

	if targetRole == "admin" {
		var adminCount int
		_ = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE role = 'admin'").Scan(&adminCount)
		if adminCount <= 1 {
			return errors.New("系统中至少需要保留一个管理员账号")
		}
	}

	_, err = s.db.ExecContext(ctx, "DELETE FROM users WHERE id = ?", targetUserID)
	return err
}

// AdminResetPassword resets a target user's password without needing their old password
func (s *Service) AdminResetPassword(ctx context.Context, targetUserID int64, newPassword string) error {
	if len(newPassword) < 6 {
		return errors.New("新密码长度至少需要 6 个字符")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	res, err := s.db.ExecContext(ctx, "UPDATE users SET password_hash = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", string(hash), targetUserID)
	if err != nil {
		return err
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return errors.New("用户不存在")
	}

	return nil
}

func (s *Service) HandleListUsers(c *gin.Context) {
	users, err := s.ListUsers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"users": users})
}

func (s *Service) HandleCreateUser(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	u, err := s.CreateUser(c.Request.Context(), req.Username, req.Password, req.Role)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"user":    u,
		"message": "用户创建成功",
	})
}

func (s *Service) HandleDeleteUser(c *gin.Context) {
	idStr := c.Param("id")
	targetID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	currentUIDVal, _ := c.Get("user_id")
	var currentUID int64
	switch v := currentUIDVal.(type) {
	case int64:
		currentUID = v
	case string:
		currentUID, _ = strconv.ParseInt(v, 10, 64)
	}

	if err := s.DeleteUser(c.Request.Context(), currentUID, targetID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "用户删除成功"})
}

func (s *Service) HandleAdminResetPassword(c *gin.Context) {
	idStr := c.Param("id")
	targetID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	var req struct {
		NewPassword string `json:"new_password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request payload"})
		return
	}

	if err := s.AdminResetPassword(c.Request.Context(), targetID, req.NewPassword); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "密码重置成功"})
}
