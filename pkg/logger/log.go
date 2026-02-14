package logger

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
)

const (
	stepLogIDKey  = "step_log_id"
	debugMode     = "debug"
	infoMode      = "info"
	errorMode     = "error"
	defaultChunk  = 4096
	envStepEnable = "STEP_LOG_ENABLED"
	envStepLength = "STEP_LOG_LENGTH"
)

var (
	defaultLogger *slog.Logger
)

func init() {
	defaultLogger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
		AddSource: false,
	}))
}

type contextKey struct{}

var loggerKey = &contextKey{}

// FromContext returns the logger from context, or the default logger.
func FromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(loggerKey).(*slog.Logger); ok && l != nil {
		return l
	}
	return defaultLogger
}

// WithContext returns a new context that carries the given logger.
func WithContext(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, l)
}

// WithRequestID returns a new context whose logger includes the given request/step ID.
func WithRequestID(ctx context.Context, id string) context.Context {
	l := FromContext(ctx).With(stepLogIDKey, id)
	return WithContext(ctx, l)
}

// StepLogWithContext logs a message, optionally chunked by STEP_LOG_LENGTH when STEP_LOG_ENABLED is true.
func StepLogWithContext(ctx context.Context, logLevel string, messagePrefix string, msg ...interface{}) {
	message := fmt.Sprint(msg...)
	isEnabled := getBoolEnv(envStepEnable, false)
	logLength := getIntEnv(envStepLength, defaultChunk)
	length := len(message)
	chunks := length / logLength
	if length%logLength > 0 {
		chunks++
	}

	l := FromContext(ctx)
	if chunks <= 1 || !isEnabled {
		logChunk(l, ctx, logLevel, messagePrefix, message)
		return
	}
	for i := 0; i < chunks; i++ {
		end := (i + 1) * logLength
		if end > length {
			end = length
		}
		chunk := message[i*logLength : end]
		logChunk(l, ctx, logLevel, fmt.Sprintf("%s [%d]", messagePrefix, i+1), chunk)
	}
}

func logChunk(l *slog.Logger, ctx context.Context, logLevel, prefix, message string) {
	msg := prefix + " :- " + message
	switch logLevel {
	case debugMode:
		l.DebugContext(ctx, msg)
	case infoMode:
		l.InfoContext(ctx, msg)
	case errorMode:
		l.ErrorContext(ctx, msg)
	default:
		l.WarnContext(ctx, "Unhandled log level: "+logLevel+". Message: "+message)
	}
}

// DebugfWithContext logs at debug level with format.
func DebugfWithContext(ctx context.Context, format string, args ...interface{}) {
	FromContext(ctx).DebugContext(ctx, fmt.Sprintf(format, args...))
}

// InfofWithContext logs at info level with format.
func InfofWithContext(ctx context.Context, format string, args ...interface{}) {
	FromContext(ctx).InfoContext(ctx, fmt.Sprintf(format, args...))
}

// ErrorfWithContext logs at error level with format.
func ErrorfWithContext(ctx context.Context, format string, args ...interface{}) {
	FromContext(ctx).ErrorContext(ctx, fmt.Sprintf(format, args...))
}

// WarnfWithContext logs at warn level with format.
func WarnfWithContext(ctx context.Context, format string, args ...interface{}) {
	FromContext(ctx).WarnContext(ctx, fmt.Sprintf(format, args...))
}

// Error logs with error level. args are alternating key-value pairs (e.g. "error", err).
func Error(ctx context.Context, message string, args ...interface{}) {
	FromContext(ctx).ErrorContext(ctx, message, argsToAny(args)...)
}

// Info logs with info level. args are alternating key-value pairs.
func Info(ctx context.Context, message string, args ...interface{}) {
	FromContext(ctx).InfoContext(ctx, message, argsToAny(args)...)
}

// Debug logs with debug level. args are alternating key-value pairs.
func Debug(ctx context.Context, message string, args ...interface{}) {
	FromContext(ctx).DebugContext(ctx, message, argsToAny(args)...)
}

// Warn logs with warn level. args are alternating key-value pairs.
func Warn(ctx context.Context, message string, args ...interface{}) {
	FromContext(ctx).WarnContext(ctx, message, argsToAny(args)...)
}

func argsToAny(args []interface{}) []interface{} {
	return args
}

func getBoolEnv(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, _ := strconv.ParseBool(v)
	return b
}

func getIntEnv(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, _ := strconv.Atoi(v)
	if n <= 0 {
		return def
	}
	return n
}
