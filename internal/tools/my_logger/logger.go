package my_logger

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"sync"
)

type LoggerOptions struct {
	AppName      string
	Version      string
	Level        string
	Output       io.Writer
	DefaultAttrs map[string]any
	Serializer   Serializer
}

type Logger struct {
	logger        *slog.Logger
	contextLogger *slog.Logger
	options       *LoggerOptions
	ctxFence      sync.Mutex
}

const (
	levelTrace    = slog.Level(-8)
	levelFatal    = slog.Level(12)
	levelCritical = slog.Level(16)
)

var levelNames = map[slog.Leveler]string{
	levelTrace:    "TRACE",
	levelFatal:    "FATAL",
	levelCritical: "CRITICAL",
}

func NewLogger(opts *LoggerOptions) (*Logger, error) {
	var baseAttrs []any
	var level slog.Level
	var handlerOpts *slog.HandlerOptions

	level, err := getSlogLevel(opts.Level)
	if err != nil {
		return nil, err
	}

	if opts.Output == nil {
		opts.Output = os.Stdout
	}

	if opts.DefaultAttrs == nil {
		opts.DefaultAttrs = make(map[string]any)
	}

	if opts.Serializer == nil {
		opts.Serializer = &DefaultSerializers{}
	}

	baseAttrs = setupBaseAttrs(opts.AppName, opts.Version, opts.DefaultAttrs)

	handlerOpts = &slog.HandlerOptions{
		Level:       level,
		ReplaceAttr: replaceCustomLevelNames,
	}

	handler := slog.NewJSONHandler(opts.Output, handlerOpts)

	logger := slog.New(handler)
	logger = logger.With(baseAttrs...)

	return &Logger{
		logger:        logger,
		contextLogger: logger,
		options:       opts,
	}, nil
}

func (l *Logger) Trace(msg string, attrs ...any) {
	l.Log(context.TODO(), "trace", msg, attrs...)
}

func (l *Logger) Debug(msg string, attrs ...any) {
	l.Log(context.TODO(), "debug", msg, attrs...)
}

func (l *Logger) Info(msg string, attrs ...any) {
	l.Log(context.TODO(), "info", msg, attrs...)
}

func (l *Logger) Warn(msg string, attrs ...any) {
	l.Log(context.TODO(), "warn", msg, attrs...)
}

func (l *Logger) Error(msg string, attrs ...any) {
	l.Log(context.TODO(), "error", msg, attrs...)
}

func (l *Logger) Critical(msg string, attrs ...any) {
	l.Log(context.TODO(), "critical", msg, attrs...)
}

func (l *Logger) Fatal(msg string, attrs ...any) {
	l.Log(context.TODO(), "fatal", msg, attrs...)
}

func (l *Logger) Log(ctx context.Context, level string, msg string, attrs ...any) error {
	var slogLevel slog.Level
	slogLevel, err := getSlogLevel(level)
	if err != nil {
		return err
	}

	if len(attrs) == 1 {
		if serializedAttr, ok := l.options.Serializer.Serialize(attrs[0]); ok {
			attrs[0] = serializedAttr
		}
	}

	l.contextLogger.Log(ctx, slogLevel, msg, attrs...)

	return nil
}

func (l *Logger) AddLogContext(attrs ...any) {
	l.ctxFence.Lock()
	defer l.ctxFence.Unlock()

	l.contextLogger = l.contextLogger.With(attrs...)
}

func (l *Logger) ClearLogContext() {
	l.ctxFence.Lock()
	defer l.ctxFence.Unlock()

	l.contextLogger = l.logger
}

func (l *Logger) GetBaseLogger() *slog.Logger {
	loggerCopy := *l.logger
	return &loggerCopy
}

func BuildAttrsFromMap(appAttrs map[string]any) []any {
	attrs := make([]any, 0, len(appAttrs))

	for k, v := range appAttrs {
		slogAttr := slog.Attr{
			Key:   k,
			Value: slog.AnyValue(v),
		}

		attrs = append(attrs, slogAttr)
	}

	return attrs
}

func setupBaseAttrs(appName string, version string, defaultAttrs map[string]any) []any {
	var baseAttrs []any

	defaultAttrs["name"] = appName
	defaultAttrs["version"] = version
	defaultAttrs["hostname"], _ = os.Hostname()

	baseAttrs = BuildAttrsFromMap(defaultAttrs)

	return baseAttrs
}

func getSlogLevel(optLevel string) (slog.Level, error) {
	switch optLevel {
	case "trace":
		return levelTrace, nil
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	case "critical":
		return levelFatal, nil
	case "fatal":
		return levelCritical, nil
	default:
		return -99, errors.New("unknown level")
	}
}

func replaceCustomLevelNames(groups []string, a slog.Attr) slog.Attr {
	if a.Key == slog.LevelKey {
		level := a.Value.Any().(slog.Level)
		levelLabel, exists := levelNames[level]
		if !exists {
			levelLabel = level.String()
		}
		a.Value = slog.StringValue(levelLabel)
	}

	return a
}
