package logger

import (
	"io"
	"log/slog"
	"os"

	"github.com/bermr/api-golang-base/internal/config"
	"github.com/bermr/api-golang-base/internal/tools/my_logger"
)

func GetLogger(cfg *config.Config, output io.Writer) *my_logger.Logger {
	logger, err := my_logger.NewLogger(&my_logger.LoggerOptions{
		AppName: cfg.AppName,
		Version: "1.0.1",
		Level:   cfg.LogLevel,
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

	firehoseLogStream, err := my_logger.NewFirehoseLogStream(my_logger.FirehoseLogStreamOptions{
		StreamName: cfg.AppName,
	})
	if err != nil {
		slog.Info("firehose stream creation error", "err", err)
		panic(err)
	}

	return firehoseLogStream
}
