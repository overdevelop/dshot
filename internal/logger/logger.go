package logger

import (
	"context"
	"log/slog"
	"os"
)

const (
	LogLevelDebug = slog.LevelDebug
	LogLevelInfo  = slog.LevelInfo
	LogLevelWarn  = slog.LevelWarn
	LogLevelError = slog.LevelError
	LogLevelPanic = slog.Level(12)

	CallerSourceKey = "callerSource"
)

var (
	levelVar      slog.LevelVar
	defaultLogger *slog.Logger
	thisPkgPrefix string
	isProd        bool
	isDebug       bool
)

func init() {
	isDebug = envVarBool("DEBUG")

	if isDebug {
		levelVar.Set(slog.LevelDebug)
	} else {
		levelVar.Set(slog.LevelInfo)
	}

	handlerOptions := &slog.HandlerOptions{
		AddSource: isDebug,
		Level:     &levelVar,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.LevelKey {
				level := a.Value.Any().(slog.Level)

				switch {
				case level < LogLevelInfo:
					a.Value = slog.StringValue("DEBUG")
				case level < LogLevelWarn:
					a.Value = slog.StringValue("INFO")
				case level < LogLevelError:
					a.Value = slog.StringValue("WARN")
				case level < LogLevelPanic:
					a.Value = slog.StringValue("ERROR")
				default:
					a.Value = slog.StringValue("PANIC")
				}
			}

			return a
		},
	}

	if isProd {
		defaultLogger = slog.New(slog.NewJSONHandler(os.Stdout, handlerOptions))
	} else {
		defaultLogger = slog.New(slog.NewTextHandler(os.Stdout, handlerOptions))
	}

	slog.SetDefault(defaultLogger)
}

func Default() *slog.Logger {
	return defaultLogger
}

func Debug(msg string, args ...any) {
	defaultLogger.Debug(msg, args...)
}

func Info(msg string, args ...any) {
	defaultLogger.Info(msg, args...)
}

func Warn(msg string, args ...any) {
	defaultLogger.Warn(msg, args...)
}

func Error(msg string, args ...any) {
	defaultLogger.Error(msg, args...)
}

func Panic(msg string, args ...any) {
	defaultLogger.Log(context.Background(), LogLevelPanic, msg, args...)
}

func With(args ...any) *slog.Logger {
	return defaultLogger.With(args...)
}

func WithGroup(name string) *slog.Logger {
	return defaultLogger.WithGroup(name)
}

func SetLevel(level slog.Level) {
	levelVar.Set(level)
}

func envVar(name string) string {
	return os.Getenv(name)
}

func envVarBool(name string) bool {
	return envVar(name) == "true"
}
