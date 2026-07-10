package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	HTTPAddr        string
	DatabaseURL     string
	OpenAIBaseURL   string
	OpenAIAPIKey    string
	TinyModel       string
	MainModel       string
	ThinkingModel   string
	AccessToken     string
	DefaultUserName string
	AutoMigrate     bool
	LogLevel        slog.Level
}

func Load() Config {
	return Config{
		HTTPAddr:        env("REMVENTORY_HTTP_ADDR", ":8080"),
		DatabaseURL:     env("DATABASE_URL", ""),
		OpenAIBaseURL:   env("OPENAI_BASE_URL", "http://localhost:11434/v1"),
		OpenAIAPIKey:    env("OPENAI_API_KEY", ""),
		TinyModel:       env("OPENAI_TINY_MODEL", ""),
		MainModel:       env("OPENAI_MAIN_MODEL", env("OPENAI_MODEL", "")),
		ThinkingModel:   env("OPENAI_THINKING_MODEL", env("OPENAI_MODEL", "")),
		AccessToken:     env("REMVENTORY_ACCESS_TOKEN", ""),
		DefaultUserName: env("REMVENTORY_DEFAULT_USER_NAME", "Remventory User"),
		AutoMigrate:     envBool("REMVENTORY_AUTO_MIGRATE", true),
		LogLevel:        envLogLevel("REMVENTORY_LOG_LEVEL", slog.LevelInfo),
	}
}

func (c Config) PublicStatus() map[string]any {
	return map[string]any{
		"http_addr":                 c.HTTPAddr,
		"database_configured":       c.DatabaseURL != "",
		"model_configured":          c.MainModel != "" && c.ThinkingModel != "",
		"tiny_model_configured":     c.TinyModel != "",
		"main_model_configured":     c.MainModel != "",
		"thinking_model_configured": c.ThinkingModel != "",
		"openai_base_url":           c.OpenAIBaseURL,
		"access_token_gate":         c.AccessToken != "",
		"auto_migrate":              c.AutoMigrate,
	}
}

func env(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envLogLevel(key string, fallback slog.Level) slog.Level {
	switch strings.ToLower(env(key, "")) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return fallback
	}
}
