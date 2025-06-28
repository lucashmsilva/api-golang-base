package main

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/bermr/api-golang-base/internal/config"
	"github.com/bermr/api-golang-base/internal/infra/server"
	"github.com/bermr/api-golang-base/internal/middlewares"
	"github.com/bermr/api-golang-base/internal/tools/logger"
	"github.com/go-chi/chi/v5"
)

// TODO: implement unit tests and coverage for the main app
// TODO: implement unit tests for the logging module
// TODO: learn and implement dependency injection
// TODO: setup the logger as an injected dependency
// TODO: implement database configuration

func main() {
	var isShuttingDown atomic.Bool
	var loggerOutputStream io.Writer

	var loggerMdw *middlewares.RequestLoggerMiddleware
	var errorMdw *middlewares.ErrorMiddleware

	// loads config
	config, err := config.LoadConfig()
	if err != nil {
		slog.Error(fmt.Sprintf("Error loading config: %v", err))
		return
	}

	// setups a base slog.Logger from the custom logger
	loggerOutputStream = logger.GetOutputStream(config)
	slog.SetDefault(logger.GetLogger(config, loggerOutputStream).GetBaseLogger())

	router := chi.NewRouter()

	// middleware setup
	loggerMdw = middlewares.NewLoggerMiddleware(config, loggerOutputStream)
	errorMdw = middlewares.NewErrorMiddleware()

	// scopes a log context for the current request
	router.Use(loggerMdw.HandleRequest)

	// catch-all for in-request panics
	router.Use(errorMdw.HandleRequest)

	// app routes
	router.Handle("GET /healthcheck", healthcheckHandler())

	// starts the HTTP serve
	srv := server.New(config, router)
	go srv.Start()

	// waits for shutdown signals
	srv.GracefulShutdown(&isShuttingDown)

	// checks if the log output is a io.Closer and closes it if so
	if loggerOutputStreamCloser, ok := loggerOutputStream.(io.Closer); ok {
		loggerOutputStreamCloser.Close()
	}
}

func healthcheckHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(time.Second * 2)
		// panic(errors.New("hi"))
		fmt.Fprintln(w, "hi")
	})
}
