package account

import "time"

type Account struct {
	ID               int64      `json:"id"`
	Name             string     `json:"name"`
	Username         string     `json:"username"`
	PasswordEnc      string     `json:"-"`
	RefreshTokenEnc  string     `json:"-"`
	AccessTokenEnc   string     `json:"-"`
	UserID           string     `json:"user_id"`
	DeviceID         string     `json:"device_id"`
	CaptchaToken     string     `json:"-"`
	ProxyURL         string     `json:"proxy_url"`
	Priority         int        `json:"priority"`
	IsEnabled        bool       `json:"is_enabled"`
	Status           string     `json:"status"` // HEALTHY, AUTH_FAILED, PROXY_FAILED, RATE_LIMITED, QUOTA_EXHAUSTED, DISABLED, COOLDOWN
	QuotaExhaustedAt *time.Time `json:"quota_exhausted_at,omitempty"`
	CooldownUntil    *time.Time `json:"cooldown_until,omitempty"`
	DailyTaskCount   int        `json:"daily_task_count"`
	LastTaskDate     string     `json:"last_task_date"`
	TotalFiles       int        `json:"total_files"`
	UsedSpace        int64      `json:"used_space"`
	TotalSpace       int64      `json:"total_space"`
	LastLoginAt      *time.Time `json:"last_login_at,omitempty"`
	LastSuccessAt    *time.Time `json:"last_success_at,omitempty"`
	LastError        string     `json:"last_error,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`

	HasPassword     bool `json:"has_password"`
	HasRefreshToken bool `json:"has_refresh_token"`
}

type CreateAccountReq struct {
	Name         string `json:"name"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	RefreshToken string `json:"refresh_token"`
	ProxyURL     string `json:"proxy_url"`
	Priority     int    `json:"priority"`
	IsEnabled    bool   `json:"is_enabled"`
}

type UpdateAccountReq struct {
	Name         *string `json:"name"`
	Username     *string `json:"username"`
	Password     *string `json:"password"`
	RefreshToken *string `json:"refresh_token"`
	ProxyURL     *string `json:"proxy_url"`
	Priority     *int    `json:"priority"`
	IsEnabled    *bool   `json:"is_enabled"`
}

type ProxyTestResult struct {
	Success   bool   `json:"success"`
	LatencyMs int64  `json:"latency_ms"`
	EgressIP  string `json:"egress_ip"`
	Error     string `json:"error,omitempty"`
}

type AccountTestResult struct {
	Success    bool   `json:"success"`
	Status     string `json:"status"`
	UsedSpace  int64  `json:"used_space"`
	TotalSpace int64  `json:"total_space"`
	LatencyMs  int64  `json:"latency_ms"`
	Error      string `json:"error,omitempty"`
}
