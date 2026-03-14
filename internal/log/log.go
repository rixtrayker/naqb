// Package log provides a lightweight leveled logger for nqb.
//
// Log output is written to ~/.naqb/nqb.log when NQB_DEBUG=1 is set.
// At INFO level and above, messages are always written to the log file
// (if it can be opened). DEBUG messages are suppressed unless NQB_DEBUG=1.
//
// Usage:
//
//	log.Debug("context built", "chapter", 3, "path", path)
//	log.Info("LLM call started", "model", model, "tokens", maxTokens)
//	log.Warn("git commit skipped", "err", err)
//	log.Error("vault load failed", "err", err)
package log

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Level represents log verbosity.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO "
	case LevelWarn:
		return "WARN "
	case LevelError:
		return "ERROR"
	default:
		return "?????"
	}
}

// logger is the package-level singleton.
type logger struct {
	mu      sync.Mutex
	out     io.Writer
	level   Level
	enabled bool // whether the log file was opened successfully
}

var std = &logger{}

func init() {
	std.level = LevelInfo

	// Always try to open the log file
	logPath := logFilePath()
	if err := os.MkdirAll(filepath.Dir(logPath), 0o750); err == nil {
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err == nil {
			std.out = f
			std.enabled = true
		}
	}

	// If NQB_DEBUG=1, drop to DEBUG level and also echo to stderr
	if os.Getenv("NQB_DEBUG") == "1" {
		std.level = LevelDebug
		if std.out != nil {
			std.out = io.MultiWriter(std.out, os.Stderr)
		} else {
			std.out = os.Stderr
			std.enabled = true
		}
	}
}

// logFilePath returns the default log file path.
func logFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".naqb", "nqb.log")
}

// write formats and emits a log line.
// args must be alternating key, value pairs.
func (lg *logger) write(level Level, msg string, args ...any) {
	if !lg.enabled {
		return
	}
	if level < lg.level {
		return
	}

	// Caller info (skip write → Debug/Info/Warn/Error → actual caller)
	_, file, line, _ := runtime.Caller(2)
	short := file
	if idx := strings.LastIndex(file, "/"); idx >= 0 {
		short = file[idx+1:]
	}

	var sb strings.Builder
	sb.WriteString(time.Now().Format("2006-01-02 15:04:05.000"))
	sb.WriteByte(' ')
	sb.WriteString(level.String())
	sb.WriteByte(' ')
	sb.WriteString(short)
	sb.WriteByte(':')
	sb.WriteString(strconv.Itoa(line))
	sb.WriteByte(' ')
	sb.WriteString(msg)

	for i := 0; i+1 < len(args); i += 2 {
		sb.WriteString(fmt.Sprintf(" %v=%v", args[i], args[i+1]))
	}
	// If odd number of args, append the last one as a bare value
	if len(args)%2 != 0 {
		sb.WriteString(fmt.Sprintf(" %v", args[len(args)-1]))
	}
	sb.WriteByte('\n')

	lg.mu.Lock()
	defer lg.mu.Unlock()
	_, _ = lg.out.Write([]byte(sb.String()))
}

// Debug logs at DEBUG level (only visible when NQB_DEBUG=1).
func Debug(msg string, args ...any) {
	std.write(LevelDebug, msg, args...)
}

// Info logs at INFO level.
func Info(msg string, args ...any) {
	std.write(LevelInfo, msg, args...)
}

// Warn logs at WARN level.
func Warn(msg string, args ...any) {
	std.write(LevelWarn, msg, args...)
}

// Error logs at ERROR level.
func Error(msg string, args ...any) {
	std.write(LevelError, msg, args...)
}

// LogPath returns the path of the active log file.
func LogPath() string {
	return logFilePath()
}
