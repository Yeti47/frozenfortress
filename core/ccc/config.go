package ccc

import (
	"database/sql"
	"encoding/json" // Added import
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

type AppConfig struct {
	DatabasePath string // Path to the database file

	MaxSignInAttempts   int // Maximum number of sign-in attempts before locking the account
	SignInAttemptWindow int // Time window in minutes for counting sign-in attempts

	RedisAddress  string // Redis server address
	RedisUser     string // Redis username
	RedisPassword string // Redis password
	RedisSize     int    // Redis connection pool size
	RedisNetwork  string // Redis network type (tcp/unix)

	SigningKey    string // Session signing key
	EncryptionKey string // Session encryption key
	KeyDir        string // Directory to store persistent key files

	WebUiPort int    // Port for the Web UI server
	LogLevel  string // Log level (Debug, Info, Warn, Error)
}

// String returns a JSON representation of the AppConfig.
func (c AppConfig) String() string {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error marshalling config to JSON: %v", err)
	}
	return string(data)
}

var DefaultConfig = AppConfig{
	DatabasePath:        filepath.Join(GetUserDataDir(), "frozenfortress.db"),
	MaxSignInAttempts:   3,
	SignInAttemptWindow: 30,
	RedisAddress:        "localhost:6379",
	RedisUser:           "",
	RedisPassword:       "",
	RedisSize:           10,
	RedisNetwork:        "tcp",
	SigningKey:          "",
	EncryptionKey:       "",
	KeyDir:              "",
	WebUiPort:           8080,   // Default Web UI port
	LogLevel:            "Info", // Default log level
}

// LoadConfigFromEnv loads the application configuration from environment variables.
// If the environment variables are not set, it falls back to default values.
func LoadConfigFromEnv() AppConfig {
	config := AppConfig{
		DatabasePath:        filepath.Join(GetUserDataDir(), "frozenfortress.db"),
		MaxSignInAttempts:   3,
		SignInAttemptWindow: 30,
		RedisAddress:        "localhost:6379",
		RedisUser:           "",
		RedisPassword:       "",
		RedisSize:           10,
		RedisNetwork:        "tcp",
		SigningKey:          "",
		EncryptionKey:       "",
		KeyDir:              "",
		WebUiPort:           8080,
		LogLevel:            "info", // Default log level
	}

	// Database configuration
	if dbPath := os.Getenv("FF_DATABASE_PATH"); dbPath != "" {
		config.DatabasePath = dbPath
	}

	// Security configuration
	if maxAttempts := os.Getenv("FF_MAX_SIGN_IN_ATTEMPTS"); maxAttempts != "" {
		if attempts, err := strconv.Atoi(maxAttempts); err == nil {
			config.MaxSignInAttempts = attempts
		}
	}
	if window := os.Getenv("FF_SIGN_IN_ATTEMPT_WINDOW"); window != "" {
		if minutes, err := strconv.Atoi(window); err == nil {
			config.SignInAttemptWindow = minutes
		}
	}

	// Redis configuration
	if redisAddr := os.Getenv("FF_REDIS_ADDRESS"); redisAddr != "" {
		config.RedisAddress = redisAddr
	}
	if redisUser := os.Getenv("FF_REDIS_USER"); redisUser != "" {
		config.RedisUser = redisUser
	}
	if redisPass := os.Getenv("FF_REDIS_PASSWORD"); redisPass != "" {
		config.RedisPassword = redisPass
	}
	if redisSize := os.Getenv("FF_REDIS_SIZE"); redisSize != "" {
		if size, err := strconv.Atoi(redisSize); err == nil && size > 0 {
			config.RedisSize = size
		}
	}
	if redisNet := os.Getenv("FF_REDIS_NETWORK"); redisNet != "" {
		config.RedisNetwork = redisNet
	}

	// Session key configuration
	if signingKey := os.Getenv("FF_SIGNING_KEY"); signingKey != "" {
		config.SigningKey = signingKey
	}
	if encKey := os.Getenv("FF_ENCRYPTION_KEY"); encKey != "" {
		config.EncryptionKey = encKey
	}
	if keyDir := os.Getenv("FF_KEY_DIR"); keyDir != "" {
		config.KeyDir = keyDir
	}

	// Web UI configuration
	if webUIPort := os.Getenv("FF_WEB_UI_PORT"); webUIPort != "" {
		if port, err := strconv.Atoi(webUIPort); err == nil {
			config.WebUiPort = port
		}
	}

	// Log level configuration
	if logLevel := os.Getenv("FF_LOG_LEVEL"); logLevel != "" {
		config.LogLevel = logLevel
	}

	return config
}

// GetUserDataDir returns the OS-specific user data directory for storing application data
func GetUserDataDir() string {
	appName := "frozenfortress"

	// Check common environment variables first
	if userConfigDir := os.Getenv("XDG_CONFIG_HOME"); userConfigDir != "" {
		// Linux XDG Base Directory Specification
		return filepath.Join(userConfigDir, appName)
	}

	if appData := os.Getenv("APPDATA"); appData != "" {
		// Windows %APPDATA%
		return filepath.Join(appData, appName)
	}

	if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
		// Windows %LOCALAPPDATA% (fallback)
		return filepath.Join(localAppData, appName)
	}

	// Platform-specific defaults
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Last resort fallback - this will be relative to the working directory
		return "./data"
	}

	// Linux/Unix: ~/.config/frozenfortress
	// macOS: ~/Library/Application Support/frozenfortress (though we use .config for consistency)
	return filepath.Join(homeDir, ".config", appName)
}

// SetupDatabase creates and configures the SQLite database
func SetupDatabase(config AppConfig) (*sql.DB, error) {
	// Ensure the directory exists
	dbDir := filepath.Dir(config.DatabasePath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Open the database connection
	db, err := sql.Open("sqlite3", config.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Enable foreign keys and other SQLite optimizations
	pragmas := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA cache_size = -64000", // 64MB cache
		"PRAGMA temp_store = MEMORY",
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to execute pragma '%s': %w", pragma, err)
		}
	}

	return db, nil
}

func CreateLogger(config AppConfig) Logger {

	var level slog.Level
	switch strings.ToLower(config.LogLevel) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		// Default to Info if unknown level
		level = slog.LevelInfo
	}

	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))
}
