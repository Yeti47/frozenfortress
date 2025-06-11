package ccc

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// dailyRotatingWriter is a writer that creates a new log file each day
type dailyRotatingWriter struct {
	logDir      string
	filename    string
	currentFile *os.File
	currentDate string
	mu          sync.Mutex
}

// newDailyRotatingWriter creates a new daily rotating writer
func newDailyRotatingWriter(logDir, filename string) *dailyRotatingWriter {
	return &dailyRotatingWriter{
		logDir:   logDir,
		filename: filename,
	}
}

// Write implements the io.Writer interface
func (w *dailyRotatingWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	currentDate := time.Now().Format("2006-01-02")

	// Check if we need to rotate (new day or no file open)
	if w.currentFile == nil || w.currentDate != currentDate {
		if err := w.rotate(currentDate); err != nil {
			return 0, err
		}
	}

	return w.currentFile.Write(p)
}

// rotate closes the current file and opens a new one for the given date
func (w *dailyRotatingWriter) rotate(date string) error {
	// Close current file if open
	if w.currentFile != nil {
		w.currentFile.Close()
	}

	// Create new filename with date
	filename := fmt.Sprintf("%s-%s.log", w.filename, date)
	filepath := filepath.Join(w.logDir, filename)

	// Open new file
	file, err := os.OpenFile(filepath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	w.currentFile = file
	w.currentDate = date
	return nil
}

// Close closes the current file
func (w *dailyRotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.currentFile != nil {
		return w.currentFile.Close()
	}
	return nil
}

// CreateLogger creates a logger that writes to daily rotating log files
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

	// Create log directory if it doesn't exist
	logDir := filepath.Join(GetUserDataDir(), "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		// Fallback to console logging if we can't create the log directory
		return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: level,
		}))
	}

	// Create daily rotating writer
	rotatingWriter := newDailyRotatingWriter(logDir, "frozenfortress")

	return slog.New(slog.NewJSONHandler(rotatingWriter, &slog.HandlerOptions{
		Level: level,
	}))
}
