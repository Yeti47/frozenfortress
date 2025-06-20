package ccc

import (
	"database/sql"
	"encoding/json" // Added import
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// Environment variable names
const (
	EnvDatabasePath         = "FF_DATABASE_PATH"
	EnvMaxSignInAttempts    = "FF_MAX_SIGN_IN_ATTEMPTS"
	EnvSignInAttemptWindow  = "FF_SIGN_IN_ATTEMPT_WINDOW"
	EnvRedisAddress         = "FF_REDIS_ADDRESS"
	EnvRedisUser            = "FF_REDIS_USER"
	EnvRedisPassword        = "FF_REDIS_PASSWORD"
	EnvRedisSize            = "FF_REDIS_SIZE"
	EnvRedisNetwork         = "FF_REDIS_NETWORK"
	EnvSigningKey           = "FF_SIGNING_KEY"
	EnvEncryptionKey        = "FF_ENCRYPTION_KEY"
	EnvKeyDir               = "FF_KEY_DIR"
	EnvWebUIPort            = "FF_WEB_UI_PORT"
	EnvLogLevel             = "FF_LOG_LEVEL"
	EnvBackupEnabled        = "FF_BACKUP_ENABLED"
	EnvBackupIntervalDays   = "FF_BACKUP_INTERVAL_DAYS"
	EnvBackupDirectory      = "FF_BACKUP_DIRECTORY"
	EnvBackupMaxGenerations = "FF_BACKUP_MAX_GENERATIONS"
	EnvOCRLanguages         = "FF_OCR_LANGUAGES"
)

// BackupConfig contains all backup-related configuration settings
type BackupConfig struct {
	Enabled        bool   // Enable/disable backup functionality
	IntervalDays   int    // Backup interval in days (0 = disabled)
	Directory      string // Directory where backup files are stored
	MaxGenerations int    // Maximum number of backup files to keep
}

// OCRConfig contains OCR-related configuration settings
type OCRConfig struct {
	Languages []string // OCR languages to use (e.g., ["eng", "deu"] for English and German)
}

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

	Backup BackupConfig // Backup configuration
	OCR    OCRConfig    // OCR configuration
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
	Backup: BackupConfig{
		Enabled:        false,                                      // Disabled by default
		IntervalDays:   7,                                          // Weekly backups
		Directory:      filepath.Join(GetUserDataDir(), "backups"), // Default backup directory
		MaxGenerations: 10,                                         // Keep 10 backup generations
	},
	OCR: OCRConfig{
		Languages: []string{"eng"}, // English by default
	},
}

// LoadConfigFromEnv loads the application configuration from environment variables.
// If the environment variables are not set, it falls back to default values.
func LoadConfigFromEnv() AppConfig {
	config := DefaultConfig

	// Database configuration
	if dbPath := os.Getenv(EnvDatabasePath); dbPath != "" {
		config.DatabasePath = dbPath
	}

	// Security configuration
	if maxAttempts := os.Getenv(EnvMaxSignInAttempts); maxAttempts != "" {
		if attempts, err := strconv.Atoi(maxAttempts); err == nil {
			config.MaxSignInAttempts = attempts
		}
	}
	if window := os.Getenv(EnvSignInAttemptWindow); window != "" {
		if minutes, err := strconv.Atoi(window); err == nil {
			config.SignInAttemptWindow = minutes
		}
	}

	// Redis configuration
	if redisAddr := os.Getenv(EnvRedisAddress); redisAddr != "" {
		config.RedisAddress = redisAddr
	}
	if redisUser := os.Getenv(EnvRedisUser); redisUser != "" {
		config.RedisUser = redisUser
	}
	if redisPass := os.Getenv(EnvRedisPassword); redisPass != "" {
		config.RedisPassword = redisPass
	}
	if redisSize := os.Getenv(EnvRedisSize); redisSize != "" {
		if size, err := strconv.Atoi(redisSize); err == nil && size > 0 {
			config.RedisSize = size
		}
	}
	if redisNet := os.Getenv(EnvRedisNetwork); redisNet != "" {
		config.RedisNetwork = redisNet
	}

	// Session key configuration
	if signingKey := os.Getenv(EnvSigningKey); signingKey != "" {
		config.SigningKey = signingKey
	}
	if encKey := os.Getenv(EnvEncryptionKey); encKey != "" {
		config.EncryptionKey = encKey
	}
	if keyDir := os.Getenv(EnvKeyDir); keyDir != "" {
		config.KeyDir = keyDir
	}

	// Web UI configuration
	if webUIPort := os.Getenv(EnvWebUIPort); webUIPort != "" {
		if port, err := strconv.Atoi(webUIPort); err == nil {
			config.WebUiPort = port
		}
	}

	// Log level configuration
	if logLevel := os.Getenv(EnvLogLevel); logLevel != "" {
		config.LogLevel = logLevel
	}

	// Backup configuration
	if backupEnabled := os.Getenv(EnvBackupEnabled); backupEnabled != "" {
		config.Backup.Enabled = backupEnabled == "true"
	}
	if backupInterval := os.Getenv(EnvBackupIntervalDays); backupInterval != "" {
		if interval, err := strconv.Atoi(backupInterval); err == nil {
			config.Backup.IntervalDays = interval
		}
	}
	if backupDir := os.Getenv(EnvBackupDirectory); backupDir != "" {
		config.Backup.Directory = backupDir
	}
	if maxGenerations := os.Getenv(EnvBackupMaxGenerations); maxGenerations != "" {
		if generations, err := strconv.Atoi(maxGenerations); err == nil {
			config.Backup.MaxGenerations = generations
		}
	}

	// OCR configuration
	if ocrLanguages := os.Getenv(EnvOCRLanguages); ocrLanguages != "" {
		// Parse comma-separated languages (e.g., "eng,deu" or "eng+deu")
		languages := strings.FieldsFunc(ocrLanguages, func(c rune) bool {
			return c == ',' || c == '+' || c == ' '
		})
		// Filter out empty strings and trim whitespace
		var cleanLanguages []string
		for _, lang := range languages {
			if trimmed := strings.TrimSpace(lang); trimmed != "" {
				cleanLanguages = append(cleanLanguages, trimmed)
			}
		}
		if len(cleanLanguages) > 0 {
			config.OCR.Languages = cleanLanguages
		}
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

	// Enable foreign keys
	pragmas := []string{
		"PRAGMA foreign_keys = ON",
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to execute pragma '%s': %w", pragma, err)
		}
	}

	return db, nil
}
