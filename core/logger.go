package core

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
)

// LogLevel represents the logging level
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

// Logger represents a logger with levels
type Logger struct {
	mu      sync.RWMutex
	level   LogLevel
	verbose bool
}

var defaultLogger = &Logger{
	level:   INFO,
	verbose: false,
}

// SetLogLevel sets the default log level
func SetLogLevel(level LogLevel) {
	defaultLogger.mu.Lock()
	defer defaultLogger.mu.Unlock()
	defaultLogger.level = level
}

// SetVerbose enables or disables verbose logging
func SetVerbose(enabled bool) {
	defaultLogger.mu.Lock()
	defer defaultLogger.mu.Unlock()
	defaultLogger.verbose = enabled
}

// SetLogLevelString sets the log level from a string
func SetLogLevelString(level string) {
	switch strings.ToLower(level) {
	case "debug":
		SetLogLevel(DEBUG)
	case "info":
		SetLogLevel(INFO)
	case "warn", "warning":
		SetLogLevel(WARN)
	case "error":
		SetLogLevel(ERROR)
	default:
		SetLogLevel(INFO)
	}
}

// GetLogLevel returns the current log level
func GetLogLevel() LogLevel {
	defaultLogger.mu.RLock()
	defer defaultLogger.mu.RUnlock()
	return defaultLogger.level
}

// LogDebug logs a debug message
func LogDebug(format string, v ...interface{}) {
	defaultLogger.log(DEBUG, format, v...)
}

// LogInfo logs an info message
func LogInfo(format string, v ...interface{}) {
	defaultLogger.log(INFO, format, v...)
}

// LogWarn logs a warning message
func LogWarn(format string, v ...interface{}) {
	defaultLogger.log(WARN, format, v...)
}

// LogError logs an error message
func LogError(format string, v ...interface{}) {
	defaultLogger.log(ERROR, format, v...)
}

// log logs a message at the specified level
func (l *Logger) log(level LogLevel, format string, v ...interface{}) {
	l.mu.RLock()
	currentLevel := l.level
	verbose := l.verbose
	l.mu.RUnlock()

	// Only log if level is appropriate
	if level < currentLevel {
		return
	}

	// Format message
	message := fmt.Sprintf(format, v...)
	
	// Add level prefix
	prefix := l.levelPrefix(level)
	
	// Add timestamp and level
	formatted := fmt.Sprintf("%s %s", prefix, message)
	
	// Log to stdout or stderr
	if level >= ERROR {
		log.Printf("%s", formatted)
	} else {
		log.Printf("%s", formatted)
	}
	
	// Verbose mode: also log to file
	if verbose {
		l.logToFile(level, formatted)
	}
}

// levelPrefix returns the prefix for a log level
func (l *Logger) levelPrefix(level LogLevel) string {
	switch level {
	case DEBUG:
		return "🔍 DEBUG"
	case INFO:
		return "✅ INFO"
	case WARN:
		return "⚠️  WARN"
	case ERROR:
		return "❌ ERROR"
	default:
		return "ℹ️  INFO"
	}
}

// logToFile logs to a file (for verbose mode)
func (l *Logger) logToFile(level LogLevel, message string) {
	// In a production system, this would write to a file
	// For now, we just ensure the message is formatted
	_ = level
	_ = message
}

// DisableDefaultLogging disables the default Go logger output
func DisableDefaultLogging() {
	log.SetOutput(os.Stderr)
	log.SetPrefix("")
	log.SetFlags(0)
}

// EnableDefaultLogging enables the default Go logger
func EnableDefaultLogging() {
	log.SetFlags(log.LstdFlags)
}

