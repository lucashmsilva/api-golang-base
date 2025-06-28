package logger

import (
	"io"
	"log/slog"
	"os"

	"github.com/bermr/api-golang-base/internal/config"
	"github.com/bermr/api-golang-base/pkg/yall"
)

func GetLogger(cfg *config.Config, output io.Writer) *yall.Logger {
	logger, err := yall.NewLogger(&yall.LoggerOptions{
		AppName: cfg.AppName,
		Version: cfg.Version,
		Level:   cfg.Log.Level,
		Output:  output,
	})

	if err != nil {
		slog.Info("logger creation error", "err", err)
		panic(err)
	}

	return logger
}

func OutputStream(cfg *config.Config) io.Writer {
	if cfg.Env == "development" {
		return os.Stdout
	}

	firehoseLogStream, err := yall.NewFirehoseLogStream(&yall.FirehoseLogStreamOptions{
		StreamName: cfg.Log.StreamName,
	})
	if err != nil {
		slog.Info("firehose stream creation error", "err", err)
		panic(err)
	}

	return firehoseLogStream
}
