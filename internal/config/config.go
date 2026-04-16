package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config contains runtime options for the Go API service.
type Config struct {
	Address               string
	DBDriver              string
	DBPath                string
	DatabaseURL           string
	LogLevel              slog.Level
	ViewsRateLimit        int
	ViewsRateWindow       time.Duration
	ShutdownTimeout       time.Duration
	RequestBodyLimitBytes int64
	TrustProxyHeaders     bool
	AllowedOrigins        []string
}

func Load() (Config, error) {
	cfg := Config{
		Address:               resolveAddress(),
		DBDriver:              strings.ToLower(getEnv("GO_API_DB_DRIVER", "sqlite")),
		DBPath:                getEnv("GO_API_DB_PATH", "data/lazzerex.sqlite"),
		DatabaseURL:           getEnv("GO_API_DATABASE_URL", ""),
		ViewsRateLimit:        30,
		ViewsRateWindow:       time.Minute,
		ShutdownTimeout:       10 * time.Second,
		RequestBodyLimitBytes: 64 * 1024,
		TrustProxyHeaders:     getEnvBool("GO_API_TRUST_PROXY_HEADERS", false),
		AllowedOrigins: getEnvStringSlice("GO_API_ALLOWED_ORIGINS", []string{
			"http://localhost:4321",
			"http://127.0.0.1:4321",
		}),
	}

	if level, err := parseLogLevel(getEnv("GO_API_LOG_LEVEL", "info")); err != nil {
		return cfg, err
	} else {
		cfg.LogLevel = level
	}

	if value, err := getEnvInt("GO_API_VIEWS_RATE_LIMIT", cfg.ViewsRateLimit); err != nil {
		return cfg, err
	} else {
		cfg.ViewsRateLimit = value
	}

	if value, err := getEnvDuration("GO_API_VIEWS_RATE_WINDOW", cfg.ViewsRateWindow); err != nil {
		return cfg, err
	} else {
		cfg.ViewsRateWindow = value
	}

	if value, err := getEnvDuration("GO_API_SHUTDOWN_TIMEOUT", cfg.ShutdownTimeout); err != nil {
		return cfg, err
	} else {
		cfg.ShutdownTimeout = value
	}

	if value, err := getEnvInt64("GO_API_REQUEST_BODY_LIMIT_BYTES", cfg.RequestBodyLimitBytes); err != nil {
		return cfg, err
	} else {
		cfg.RequestBodyLimitBytes = value
	}

	if cfg.ViewsRateLimit <= 0 {
		return cfg, fmt.Errorf("GO_API_VIEWS_RATE_LIMIT must be > 0")
	}

	if cfg.ViewsRateWindow <= 0 {
		return cfg, fmt.Errorf("GO_API_VIEWS_RATE_WINDOW must be > 0")
	}

	if cfg.ShutdownTimeout <= 0 {
		return cfg, fmt.Errorf("GO_API_SHUTDOWN_TIMEOUT must be > 0")
	}

	if cfg.RequestBodyLimitBytes <= 0 {
		return cfg, fmt.Errorf("GO_API_REQUEST_BODY_LIMIT_BYTES must be > 0")
	}

	if cfg.DBDriver != "sqlite" && cfg.DBDriver != "postgres" {
		return cfg, fmt.Errorf("GO_API_DB_DRIVER must be one of: sqlite, postgres")
	}

	if cfg.DBDriver == "sqlite" && strings.TrimSpace(cfg.DBPath) == "" {
		return cfg, fmt.Errorf("GO_API_DB_PATH is required when GO_API_DB_DRIVER=sqlite")
	}

	if cfg.DBDriver == "postgres" && strings.TrimSpace(cfg.DatabaseURL) == "" {
		return cfg, fmt.Errorf("GO_API_DATABASE_URL is required when GO_API_DB_DRIVER=postgres")
	}

	return cfg, nil
}

func parseLogLevel(raw string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info", "":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("unsupported GO_API_LOG_LEVEL: %q", raw)
	}
}

func resolveAddress() string {
	if explicitAddress := strings.TrimSpace(os.Getenv("GO_API_ADDRESS")); explicitAddress != "" {
		return explicitAddress
	}

	if servicePort := strings.TrimSpace(os.Getenv("PORT")); servicePort != "" {
		return ":" + servicePort
	}

	return ":8080"
}

func getEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	return value
}

func getEnvBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	switch strings.ToLower(value) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func getEnvInt(key string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid integer for %s: %w", key, err)
	}

	return parsed, nil
}

func getEnvInt64(key string, fallback int64) (int64, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid int64 for %s: %w", key, err)
	}

	return parsed, nil
}

func getEnvDuration(key string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid duration for %s: %w", key, err)
	}

	return parsed, nil
}

func getEnvStringSlice(key string, fallback []string) []string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return append([]string(nil), fallback...)
	}

	rawItems := strings.Split(value, ",")
	items := make([]string, 0, len(rawItems))
	for _, rawItem := range rawItems {
		item := strings.TrimSpace(rawItem)
		if item == "" {
			continue
		}

		if item != "*" {
			item = strings.TrimRight(item, "/")
		}

		if item != "" {
			items = append(items, item)
		}
	}

	if len(items) == 0 {
		return append([]string(nil), fallback...)
	}

	return items
}
