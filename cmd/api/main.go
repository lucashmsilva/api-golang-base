package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"

	"github.com/bermr/api-golang-base/internal/config"
	"github.com/bermr/api-golang-base/internal/infra/server"
	"github.com/gorilla/mux"
)

/*
	TODO: setup config load from AWS SSM
	TODO: setup logger with kinesis handler
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

	slog.Info(fmt.Sprintf("Config loaded successfully: %+v\n", config))

	srv.GracefulShutdown(&isShuttingDown)
}
