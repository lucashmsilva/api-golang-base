package gn_logger

import (
	"io"
	"log/slog"
)

type LoggerOptions struct {
	appName      string
	version      string
	level        string
	output       io.Writer
	prettyPrint  bool
	defaultAttrs map[string]interface{}
}

type Logger struct {
	logger slog.Logger
}
