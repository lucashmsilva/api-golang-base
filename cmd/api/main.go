package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"

	"github.com/bermr/api-golang-base/internal/config"
	"github.com/bermr/api-golang-base/internal/infra/server"
	"github.com/bermr/api-golang-base/internal/infra/server/gn_logger"
	"github.com/gorilla/mux"
)

/*
	TODO: validate unit tests for the logging module
	TODO: extract the logging module as its own lib
	TODO: learn and implement dependency injection
	TODO: setup the logger as an injected dependency
*/

func main() {
	var isShuttingDown atomic.Bool
	config, err := config.LoadConfig()
	if err != nil {
		slog.Error(fmt.Sprintf("Error loading config: %v", err))
		return
	}
	router := mux.NewRouter()

	router.HandleFunc("/healthcheck", func(w http.ResponseWriter, r *http.Request) {
		if isShuttingDown.Load() {
			http.Error(w, "Server shutting down", http.StatusServiceUnavailable)
			return
		}

		fmt.Fprintln(w, "OK")
	})

	srv := server.New(config, router)
	go srv.Start()

	slog.Info("Config loaded successfully")

	maxBatchSize := 1
	firehoseLogStream, err := gn_logger.NewFirehoseLogStream(gn_logger.FirehoseLogStreamOptions{
		StreamName:   "test_logs",
		MaxBatchSize: &maxBatchSize,
	})
	if err != nil {
		slog.Info("firehose stream creation error", "err", err)
	}

	logger, err := gn_logger.NewLogger(gn_logger.LoggerOptions{
		AppName: config.AppName,
		Version: "1.0.1",
		Level:   "info",
		Output:  firehoseLogStream,
	})
	if err != nil {
		slog.Info("logger creation error", "err", err)
	}

	logger.Log(context.TODO(), "info", "hello world of logs")
	logger.Log(context.TODO(), "info", "just sent my first log")
	logger.Log(context.TODO(), "info", "did you get it?")

	srv.GracefulShutdown(&isShuttingDown)
	firehoseLogStream.Close()
}
