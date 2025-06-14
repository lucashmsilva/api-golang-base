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

/*
	TODO: extract the logging module as its own lib
	TODO: implement unit tests for the logging module
	TODO: learn and implement dependency injection
	TODO: setup the logger as an injected dependency
*/

func main() {
	var isShuttingDown atomic.Bool
	var loggerOutputStream io.Writer

	var loggerMdw *middlewares.RequestLoggerMiddleware
	var errorMdw *middlewares.ErrorMiddleware

	config, err := config.LoadConfig()
	if err != nil {
		slog.Error(fmt.Sprintf("Error loading config: %v", err))
		return
	}

	loggerOutputStream = logger.OutputStream(config)
	slog.SetDefault(logger.GetLogger(config, loggerOutputStream).GetBaseLogger())

	router := chi.NewRouter()

	loggerMdw = middlewares.NewLoggerMiddleware(config, loggerOutputStream)
	errorMdw = middlewares.NewErrorMiddleware()

	// scopes a log context for the current request
	router.Use(loggerMdw.HandleRequest)

	// catch-all for in-request panics
	router.Use(errorMdw.HandleRequest)

	router.Handle("GET /healthcheck", healthcheckHandler())

	srv := server.New(config, router)
	go srv.Start()

	srv.GracefulShutdown(&isShuttingDown)

	if outStreamCloser, ok := loggerOutputStream.(io.Closer); ok {
		outStreamCloser.Close()
	}
}

func healthcheckHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(time.Second * 2)
		// panic(errors.New("hi"))
		fmt.Fprintln(w, "OK")
	})
}
