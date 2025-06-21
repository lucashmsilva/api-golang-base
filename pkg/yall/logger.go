package yall

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"sync"
)

type LoggerOptions struct {
	// Application name. Default attr that will be included in all log records.
	AppName string

	// Application version. Default attr that will be included in all log records.
	Version string

	// Minimal log level. Logs with a lesser level will not be sent to the output stream.
	Level string

	// Output stream where the records will be sent.
	Output io.Writer

	// A Optional map of attrs that will be included in in all log records along with app name, version and hostname.
	// In it's current version only a flat map (i.e. no nesting) is supported.
	DefaultAttrs map[string]any

	// Predefined serialization of some structs and interfaces. Usually, a struct is serialized as a slog.Group.
	// When sending serializable structs as attrs, send only a single attr. When there is a single attr and it is serializable,
	// opts.Serializer.Serialize(attr) is called and the resulting slog.Attr is logged.
	// The default Serializer already parses error, *http.Request and yall.*HttpResponseLogData.
	Serializer Serializer
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

// Set of methods that generates a log record with the appropriate level.
// If LoggerOptions.Serializer was set and there is only a single attr (len(attrs) == 1) and it is serializable,
// the result of Serialize(attr), an slog.Attr, is included in the record.
// The context logger is always used.
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
func (l *Logger) Fatal(msg string, attrs ...any) {
	l.Log(context.TODO(), "fatal", msg, attrs...)
}
func (l *Logger) Critical(msg string, attrs ...any) {
	l.Log(context.TODO(), "critical", msg, attrs...)
}

// Logs with the provided [level]. As of the current version, this lib does nothing with the passed [context]
// If LoggerOptions.Serializer was set and there is only a single attr (len(attrs) == 1) and it is serializable (),
// Serialize(attr) is called and the resulting slog.Attr is logged.
// The context logger is always used.
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

// Creates a new logging context with the provided attrs. It uses the underlying base slog.Logger
// to generate a new logger through a With() call. The previous base slog.Logger (with the default and base attrs) reference is preserved.
// This call is thread safe.
func (l *Logger) AddLogContext(attrs ...any) {
	l.ctxFence.Lock()
	defer l.ctxFence.Unlock()

	l.contextLogger = l.contextLogger.With(attrs...)
}

// Clears the context logger reverting it to the previous underlying base slog.Logger. This call is thread safe
func (l *Logger) ClearLogContext() {
	l.ctxFence.Lock()
	defer l.ctxFence.Unlock()

	l.contextLogger = l.logger
}

// Returns a copy of the clean base logger instance
func (l *Logger) GetBaseLogger() *slog.Logger {
	loggerCopy := *l.logger
	return &loggerCopy
}

// Returns a slice of slog.Attr, generated from the map. In in its current form,
// there is no support for nested maps.
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

func replaceCustomLevelNames(_ []string, a slog.Attr) slog.Attr {
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
