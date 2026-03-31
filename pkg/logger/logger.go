// Package logger provides a unified logging system based on slog.
// - Daemon mode: writes to rotating daily log files, retains 365 days.
// - CLI mode: outputs to terminal with color-coded log levels.
package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorGray   = "\033[90m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorCyan   = "\033[36m"
)

// Mode defines the logger operating mode.
type Mode int

const (
	ModeCLI    Mode = iota // output to terminal with colors
	ModeDaemon             // output to rotating log files
)

// Logger is the appication-level logger
type Logger struct {
	slogger *slog.Logger
	mode    Mode
	level   slog.Level
	rotator *fileRotator // only used in daemon mode
}

var (
	defaultLogger *Logger
	mu            sync.Mutex
)

// Init initializes the global logger. Call once at startup.
func Init(mode Mode, level slog.Level, logDir string) error {
	mu.Lock()
	defer mu.Unlock()
	l, err := newLogger(mode, level, logDir)
	if err != nil {
		return err
	}
	defaultLogger = l
	return nil
}

// Default returns the global logger, initializing as CLI/Info if not yet initialized.
func Default() *Logger {
	mu.Lock()
	defer mu.Unlock()
	if defaultLogger == nil {
		l, _ := newLogger(ModeCLI, slog.LevelInfo, "")
		defaultLogger = l
	}
	return defaultLogger
}

func newLogger(mode Mode, level slog.Level, logDir string) (*Logger, error) {
	l := &Logger{mode: mode, level: level}

	var handler slog.Handler
	switch mode {
	case ModeDaemon:
		rot, err := newFileRotator(logDir)
		if err != nil {
			return nil, fmt.Errorf("[logger] failed to create file rotator: %w", err)
		}
		l.rotator = rot
		handler = slog.NewJSONHandler(rot, &slog.HandlerOptions{
			Level:     level,
			AddSource: level == slog.LevelDebug,
		})
	default:
		handler = newColorHandler(os.Stdout, &slog.HandlerOptions{
			Level:     level,
			AddSource: level == slog.LevelDebug,
		})
	}

	l.slogger = slog.New(handler)
	return l, nil
}

// Close shuts down the logger (closes file handles etc.)
func (l *Logger) Close() error {
	if l.rotator != nil {
		return l.rotator.Close()
	}
	return nil
}

func (l *Logger) Debug(msg string, args ...any) { l.slogger.Debug(msg, args...) }
func (l *Logger) Info(msg string, args ...any)  { l.slogger.Info(msg, args...) }
func (l *Logger) Warn(msg string, args ...any)  { l.slogger.Warn(msg, args...) }
func (l *Logger) Error(msg string, args ...any) { l.slogger.Error(msg, args...) }

// Slogger returns the underlying *slog.Logger for use with packages that
// require a standard slog.Logger directly.
func (l *Logger) Slogger() *slog.Logger {
	return l.slogger
}

func (l *Logger) With(args ...any) *Logger {
	return &Logger{
		slogger: l.slogger.With(args...),
		mode:    l.mode,
		level:   l.level,
		rotator: l.rotator,
	}
}

// Package-level convenience functions.
func Debug(msg string, args ...any) { Default().Debug(msg, args...) }
func Info(msg string, args ...any)  { Default().Info(msg, args...) }
func Warn(msg string, args ...any)  { Default().Warn(msg, args...) }
func Error(msg string, args ...any) { Default().Error(msg, args...) }

// ============================================================
// Color Handler (CLI mode)
// ============================================================

type colorHandler struct {
	opts  *slog.HandlerOptions
	mu    sync.Mutex
	out   io.Writer
	attrs []slog.Attr
}

func newColorHandler(w io.Writer, opts *slog.HandlerOptions) *colorHandler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}
	return &colorHandler{out: w, opts: opts}
}

func (h *colorHandler) Enabled(_ context.Context, level slog.Level) bool {
	minLevel := slog.LevelInfo
	if h.opts.Level != nil {
		minLevel = h.opts.Level.Level()
	}
	return level >= minLevel
}

func (h *colorHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAtts := make([]slog.Attr, len(attrs)+len(h.attrs))
	copy(newAtts, h.attrs)
	copy(newAtts[len(h.attrs):], attrs)
	return &colorHandler{out: h.out, opts: h.opts, attrs: newAtts}
}

func (h *colorHandler) WithGroup(_ string) slog.Handler {
	return &colorHandler{out: h.out, opts: h.opts, attrs: h.attrs}
}

func (h *colorHandler) Handle(_ context.Context, r slog.Record) error {
	color, levelStr := levelColor(r.Level)
	var sb strings.Builder
	sb.WriteString(colorGray)
	sb.WriteString(r.Time.Format("2006-01-02 15:04:05"))
	sb.WriteString(colorReset)
	sb.WriteString(" ")

	sb.WriteString(color)
	sb.WriteString(fmt.Sprintf("%-5s", levelStr))
	sb.WriteString(colorReset)
	sb.WriteString(" ")

	if h.opts.AddSource && r.PC != 0 {
		frames := runtime.CallersFrames([]uintptr{r.PC})
		f, _ := frames.Next()
		sb.WriteString(colorGray)
		sb.WriteString(fmt.Sprintf("%s:%d ", filepath.Base(f.File), f.Line))
		sb.WriteString(colorReset)
	}

	sb.WriteString(r.Message)

	for _, attr := range h.attrs {
		sb.WriteString(" ")
		sb.WriteString(colorCyan)
		sb.WriteString(attr.Key)
		sb.WriteString(colorReset)
		sb.WriteString("=")
		sb.WriteString(fmt.Sprintf("%v", attr.Value))
	}

	sb.WriteString("\n")

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := fmt.Fprint(h.out, sb.String())
	return err
}

func levelColor(level slog.Level) (string, string) {
	switch {
	case level >= slog.LevelError:
		return colorRed, "ERROR"
	case level >= slog.LevelWarn:
		return colorYellow, "WARN"
	case level >= slog.LevelInfo:
		return "", "INFO"
	default:
		return colorGray, "DEBUG"
	}
}

// ============================================================
// File Rotator (Daemon mode)
// ============================================================

type fileRotator struct {
	mu      sync.Mutex
	dir     string
	current *os.File
	date    string // YYYY-MM-DD
}

func newFileRotator(dir string) (*fileRotator, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	r := &fileRotator{dir: dir}
	if err := r.rotateLocked(); err != nil {
		return nil, err
	}
	go r.scheduleMidnightRotation()
	return r, nil
}

// Write implements io.Writer, auto-rotates at midnight.
func (r *fileRotator) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	today := time.Now().Format("2006-01-02")
	if r.date != today {
		if err := r.rotateLocked(); err != nil {
			return 0, err
		}
	}
	return r.current.Write(p)
}

func (r *fileRotator) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.current != nil {
		return r.current.Close()
	}
	return nil
}

func (r *fileRotator) rotateLocked() error {
	if r.current != nil {
		_ = r.current.Close()
	}
	today := time.Now().Format("2006-01-02")
	path := filepath.Join(r.dir, fmt.Sprintf("collector-%s.log", today))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	r.current = f
	r.date = today
	go r.cleanup()
	return nil
}

func (r *fileRotator) cleanup() {
	r.mu.Lock()
	dir := r.dir
	r.mu.Unlock()

	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().AddDate(-1, 0, 0)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, "collector-") || !strings.HasSuffix(name, ".log") {
			continue
		}
		dateStr := strings.TrimSuffix(strings.TrimPrefix(name, "collector-"), ".log")
		t, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}
		if t.Before(cutoff) {
			_ = os.Remove(filepath.Join(dir, name))
		}
	}
}

func (r *fileRotator) scheduleMidnightRotation() {
	for {
		now := time.Now()
		next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 1, 0, now.Location())
		time.Sleep(time.Until(next))
		r.mu.Lock()
		_ = r.rotateLocked()
		r.mu.Unlock()
	}
}
