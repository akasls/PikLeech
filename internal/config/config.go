package config

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"os"
	"path/filepath"
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
	dataDir := getEnv("DATA_DIR", "./data")
	if os.Getenv("IN_DOCKER") == "true" || fileExists("/data") {
		dataDir = "/data"
	}

	appSecret := os.Getenv("APP_SECRET")
	if appSecret == "" {
		log.Println("[WARN] APP_SECRET not set! Using default secret for development. Set APP_SECRET for production!")
		appSecret = "pikpak-default-secret-key-32-chars!!"
	}

	adminUser := getEnv("ADMIN_USERNAME", "admin")
	adminPass := getEnv("ADMIN_PASSWORD", "admin123456")
	logLevel := getEnv("LOG_LEVEL", "INFO")

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Printf("[ERROR] Failed to create data directory %s: %v", dataDir, err)
	}

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
