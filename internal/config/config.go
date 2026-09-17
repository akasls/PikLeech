package config

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"os"
	"path/filepath"
	"strings"

	"pikpak-manager/internal/crypto"
)

type Config struct {
	Port          string
	DataDir       string
	DBPath        string
	AppSecret     string
	AdminUsername string
	AdminPassword string
	LogLevel      string
}

var Global Config

func Load() *Config {
	port := getEnv("PORT", "8080")
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		if os.Getenv("IN_DOCKER") == "true" || fileExists("/.dockerenv") {
			dataDir = "/data"
		} else {
			dataDir = "./data"
		}
	}

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Printf("[ERROR] Failed to create data directory %s: %v", dataDir, err)
	}

	appSecret := os.Getenv("APP_SECRET")
	if appSecret == "" {
		secretFile := filepath.Join(dataDir, ".secret_key")
		if data, err := os.ReadFile(secretFile); err == nil && len(strings.TrimSpace(string(data))) >= 16 {
			appSecret = strings.TrimSpace(string(data))
		} else {
			dbFile := filepath.Join(dataDir, "pikpak.db")
			if _, err := os.Stat(dbFile); err == nil {
				appSecret = crypto.LegacyDefaultSecret
				_ = os.WriteFile(secretFile, []byte(appSecret), 0600)
				log.Printf("[INFO] Existing database detected, using legacy default secret key for compatibility")
			} else {
				appSecret = GenerateRandomString(32)
				if err := os.WriteFile(secretFile, []byte(appSecret), 0600); err != nil {
					log.Printf("[WARN] Could not persist secret key to %s: %v", secretFile, err)
				} else {
					log.Printf("[INFO] Automatically generated and persisted secret key to %s", secretFile)
				}
			}
		}
	}

	adminUser := getEnv("ADMIN_USERNAME", "admin")
	adminPass := getEnv("ADMIN_PASSWORD", "admin123456")
	logLevel := getEnv("LOG_LEVEL", "INFO")

	dbPath := filepath.Join(dataDir, "pikpak.db")

	Global = Config{
		Port:          port,
		DataDir:       dataDir,
		DBPath:        dbPath,
		AppSecret:     appSecret,
		AdminUsername: adminUser,
		AdminPassword: adminPass,
		LogLevel:      logLevel,
	}

	return &Global
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return fallback
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}

func GenerateRandomString(n int) string {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "fallback-random-string-token"
	}
	return hex.EncodeToString(bytes)
}
