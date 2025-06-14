package my_logger

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
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

func setupBaseAttrs(appName string, version string, defaultAttrs map[string]any) []any {
	var baseAttrs []any

	defaultAttrs["name"] = appName
	defaultAttrs["version"] = version
	defaultAttrs["hostname"], _ = os.Hostname()

	baseAttrs = BuildLogAttrsFromMap(defaultAttrs)

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
