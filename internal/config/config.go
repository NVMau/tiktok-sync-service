package config

import (
	"os"
	"sync"

	"github.com/joho/godotenv"
)

type Config struct {
	// Server
	Port        string
	Environment string // development, production

	// Database
	DatabaseURL string
	RedisURL    string

	// Logging
	LogLevel  string // debug, info, warn, error
	LogOutput string // stdout, stderr, or file path

	// TikTok API
	TikTokAppKey        string
	TikTokAppSecret     string
	TikTokServiceID     string
	TikTokWebhookSecret string
	TikTokAPIBaseURL    string
	TikTokAuthBaseURL   string
}

var (
	cfg  *Config
	once sync.Once
)

func Load() *Config {
	once.Do(func() {
		// Load .env file if exists
		godotenv.Load()

		cfg = &Config{
			// Server
			Port:        getEnv("PORT", "8000"),
			Environment: getEnv("ENVIRONMENT", "development"),

			// Database
			DatabaseURL: getEnv("DATABASE_URL", "postgres://tiktok_sync:tiktok_sync_secret@localhost:5432/tiktok_sync_db?sslmode=disable"),
			RedisURL:    getEnv("REDIS_URL", "localhost:6379"),

			// Logging
			LogLevel:  getEnv("LOG_LEVEL", "info"),
			LogOutput: getEnv("LOG_OUTPUT", "stdout"),

			// TikTok API
			TikTokAppKey:        getEnv("TIKTOK_APP_KEY", ""),
			TikTokAppSecret:     getEnv("TIKTOK_APP_SECRET", ""),
			TikTokServiceID:     getEnv("TIKTOK_SERVICE_ID", ""),
			TikTokWebhookSecret: getEnv("TIKTOK_WEBHOOK_SECRET", ""),
			TikTokAPIBaseURL:    getEnv("TIKTOK_API_BASE_URL", "https://open-api.tiktokglobalshop.com"),
			TikTokAuthBaseURL:   getEnv("TIKTOK_AUTH_BASE_URL", "https://auth.tiktok-shops.com"),
		}
	})
	return cfg
}

func Get() *Config {
	return Load()
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
