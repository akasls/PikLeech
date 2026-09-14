package pikpak

import "time"

// Constants for PikPak client signatures and clients
const (
	AndroidClientID      = "YNxT9w7GMdWvEOKa"
	AndroidClientSecret  = "dbw2OtmVEeuUvIptb1Coyg"
	AndroidClientVersion = "1.53.2"
	AndroidPackageName   = "com.pikcloud.pikpak"
	AndroidSdkVersion    = "2.0.6.206003"

	WebClientID      = "YUMx5nI8ZU8Ap8pm"
	WebClientSecret  = "dbw2OtmVEeuUvIptb1Coyg"
	WebClientVersion = "2.0.0"

	ApiDriveBaseURL = "https://api-drive.mypikpak.net"
	ApiUserBaseURL  = "https://user.mypikpak.net"
)

var AndroidAlgorithms = []string{
	"SOP04dGzk0TNO7t7t9ekDbAmx+eq0OI1ovEx",
	"nVBjhYiND4hZ2NCGyV5beamIr7k6ifAsAbl",
	"Ddjpt5B/Cit6EDq2a6cXgxY9lkEIOw4yC1GDF28KrA",
	"VVCogcmSNIVvgV6U+AochorydiSymi68YVNGiz",
	"u5ujk5sM62gpJOsB/1Gu/zsfgfZO",
	"dXYIiBOAHZgzSruaQ2Nhrqc2im",
	"z5jUTBSIpBN9g4qSJGlidNAutX6",
	"KJE2oveZ34du/g1tiimm",
}

// Error response from PikPak API
type APIErrorResp struct {
	ErrorCode        int64  `json:"error_code"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// Signin / Token request and response
type SigninRequest struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	CaptchaToken string `json:"captcha_token,omitempty"`
}

type TokenRequest struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	GrantType    string `json:"grant_type"`
	RefreshToken string `json:"refresh_token"`
}

type TokenResponse struct {
	TokenType    string `json:"token_type"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	UserID       string `json:"sub"`
}

// Captcha init
type CaptchaTokenRequest struct {
	Action       string            `json:"action"`
	CaptchaToken string            `json:"captcha_token"`
	ClientID     string            `json:"client_id"`
	DeviceID     string            `json:"device_id"`
	Meta         map[string]string `json:"meta"`
	RedirectUri  string            `json:"redirect_uri"`
}

type CaptchaTokenResponse struct {
	CaptchaToken string `json:"captcha_token"`
	ExpiresIn    int64  `json:"expires_in"`
	URL          string `json:"url"`
}

// Storage Quota info
type AboutResponse struct {
	Quota struct {
		Limit         string `json:"limit"`
		Usage         string `json:"usage"`
		UsageInTrash  string `json:"usage_in_trash"`
		IsUnlimited   bool   `json:"is_unlimited"`
		Complimentary string `json:"complimentary"`
	} `json:"quota"`
	ExpiresAt string `json:"expires_at"`
	UserType  int    `json:"user_type"`
}

// File structures
type FileListResponse struct {
	Files         []FileItem `json:"files"`
	NextPageToken string     `json:"next_page_token"`
}

type FileItem struct {
	ID             string    `json:"id"`
	Kind           string    `json:"kind"` // "drive#file" or "drive#folder"
	Name           string    `json:"name"`
	ParentID       string    `json:"parent_id"`
	Size           string    `json:"size"`
	Hash           string    `json:"hash"`
	ThumbnailLink  string    `json:"thumbnail_link"`
	WebContentLink string    `json:"web_content_link"`
	CreatedTime    time.Time `json:"created_time"`
	ModifiedTime   time.Time `json:"modified_time"`
	MimeType       string    `json:"mime_type"`
	Medias         []Media   `json:"medias"`
}

type Media struct {
	MediaID   string `json:"media_id"`
	MediaName string `json:"media_name"`
	Video     struct {
		Height     int    `json:"height"`
		Width      int    `json:"width"`
		Duration   int    `json:"duration"`
		BitRate    int    `json:"bit_rate"`
		FrameRate  int    `json:"frame_rate"`
		VideoCodec string `json:"video_codec"`
		AudioCodec string `json:"audio_codec"`
		VideoType  string `json:"video_type"`
	} `json:"video"`
	Link struct {
		URL    string    `json:"url"`
		Token  string    `json:"token"`
		Expire time.Time `json:"expire"`
	} `json:"link"`
	ResolutionName string `json:"resolution_name"`
	IsDefault      bool   `json:"is_default"`
	IsOrigin       bool   `json:"is_origin"`
}

// Offline Download
type OfflineCreateRequest struct {
	Kind       string `json:"kind"`
	Name       string `json:"name,omitempty"`
	UploadType string `json:"upload_type"`
	URL        struct {
		URL string `json:"url"`
	} `json:"url"`
	ParentID   string `json:"parent_id,omitempty"`
	FolderType string `json:"folder_type"`
}

type OfflineDownloadResponse struct {
	File *string     `json:"file"`
	Task OfflineTask `json:"task"`
}

type OfflineListResponse struct {
	Tasks         []OfflineTask `json:"tasks"`
	NextPageToken string        `json:"next_page_token"`
}

type OfflineTask struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	UserID      string `json:"user_id"`
	FileID      string `json:"file_id"`
	FileName    string `json:"file_name"`
	FileSize    string `json:"file_size"`
	Message     string `json:"message"`
	Phase       string `json:"phase"` // PHASE_TYPE_PENDING, PHASE_TYPE_RUNNING, PHASE_TYPE_COMPLETE, PHASE_TYPE_ERROR
	Progress    int64  `json:"progress"`
	IconLink    string `json:"icon_link"`
	CreatedTime string `json:"created_time"`
	UpdatedTime string `json:"updated_time"`
}
