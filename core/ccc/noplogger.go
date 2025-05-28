package ccc

// nopLogger is a no-operation logger that implements the Logger interface.
type nopLogger struct{}

// NopLogger is a singleton Logger that performs no operations.
// Use this when no logging is desired or when a logger is required but no output is needed.
var NopLogger Logger = &nopLogger{}

// Info implements the Logger interface for nopLogger.
func (l *nopLogger) Info(msg string, args ...any) {}

// Warn implements the Logger interface for nopLogger.
func (l *nopLogger) Warn(msg string, args ...any) {}

// Error implements the Logger interface for nopLogger.
func (l *nopLogger) Error(msg string, args ...any) {}

// Debug implements the Logger interface for nopLogger.
func (l *nopLogger) Debug(msg string, args ...any) {}
