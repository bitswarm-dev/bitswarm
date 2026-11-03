package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
)

type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

func (l Level) slogLevel() slog.Level {
	switch l {
	case LevelDebug:
		return slog.LevelDebug
	case LevelWarn:
		return slog.LevelWarn
	case LevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "INFO"
	}
}

func ParseLevel(s string) Level {
	switch strings.ToLower(s) {
	case "debug":
		return LevelDebug
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

type Options struct {
	Level  Level
	Format string // "json" or "console"; anything else falls back to "console"
	Output io.Writer
}

type Logger struct {
	slog *slog.Logger
}

func New(opts Options) *Logger {
	out := opts.Output
	if out == nil {
		out = os.Stdout
	}

	levelVar := &slog.LevelVar{}
	levelVar.Set(opts.Level.slogLevel())

	var handler slog.Handler
	if strings.ToLower(opts.Format) == "json" {
		handler = slog.NewJSONHandler(out, &slog.HandlerOptions{Level: levelVar})
	} else {
		handler = newConsoleHandler(out, levelVar)
	}

	return &Logger{slog: slog.New(handler)}
}

func (l *Logger) With(args ...any) *Logger {
	return &Logger{slog: l.slog.With(args...)}
}

func (l *Logger) Debug(msg string, args ...any) { l.slog.Debug(msg, args...) }
func (l *Logger) Info(msg string, args ...any)  { l.slog.Info(msg, args...) }
func (l *Logger) Warn(msg string, args ...any)  { l.slog.Warn(msg, args...) }
func (l *Logger) Error(msg string, args ...any) { l.slog.Error(msg, args...) }

type consoleHandler struct {
	mu    *sync.Mutex
	out   io.Writer
	level slog.Leveler
	attrs []slog.Attr
}

func newConsoleHandler(out io.Writer, level slog.Leveler) *consoleHandler {
	return &consoleHandler{mu: &sync.Mutex{}, out: out, level: level}
}

func (h *consoleHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

func (h *consoleHandler) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder
	b.WriteString(r.Time.Format("15:04:05"))
	b.WriteByte(' ')
	fmt.Fprintf(&b, "%-5s", levelFromSlog(r.Level))
	b.WriteByte(' ')
	b.WriteString(r.Message)

	for _, a := range h.attrs {
		fmt.Fprintf(&b, " %s=%v", a.Key, a.Value)
	}
	r.Attrs(func(a slog.Attr) bool {
		fmt.Fprintf(&b, " %s=%v", a.Key, a.Value)
		return true
	})
	b.WriteByte('\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := io.WriteString(h.out, b.String())
	return err
}

func (h *consoleHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &consoleHandler{
		mu:    h.mu,
		out:   h.out,
		level: h.level,
		attrs: append(append([]slog.Attr{}, h.attrs...), attrs...),
	}
}

func (h *consoleHandler) WithGroup(_ string) slog.Handler {
	return h
}

func levelFromSlog(l slog.Level) string {
	switch {
	case l < slog.LevelInfo:
		return "DEBUG"
	case l < slog.LevelWarn:
		return "INFO"
	case l < slog.LevelError:
		return "WARN"
	default:
		return "ERROR"
	}
}
