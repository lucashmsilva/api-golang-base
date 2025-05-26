package server

import (
	"context"
	"fmt"
	"log/slog"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"
)

// heavily inspired by https://victoriametrics.com/blog/go-graceful-shutdown/index.html

const (
	_shutdownPeriod      = 15 * time.Second
	_shutdownHardPeriod  = 3 * time.Second
	_readinessDrainDelay = 5 * time.Second
)

func (s *Server) GracefulShutdown(shutdownFlag *atomic.Bool) context.Context {
	// Setup signal context
	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Wait for signal
	slog.Info("Listening for process termination signals.")
	<-rootCtx.Done()
	stop()
	shutdownFlag.Store(true)
	slog.Info("Received shutdown signal, shutting down.")

	// Give time for readiness check to propagate
	time.Sleep(_readinessDrainDelay)
	slog.Info("Readiness check propagated, now waiting for ongoing requests to finish.")

	// Effectively shutdown the HTTP server
	shutdownCtx, cancel := context.WithTimeout(context.Background(), _shutdownPeriod)
	defer cancel()
	err := s.Shutdown(shutdownCtx)
	s.requestStopper()
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to wait for ongoing requests to finish, waiting for forced cancellation.: %v", err))
		time.Sleep(_shutdownHardPeriod)
	}

	slog.Info("Server shutdown gracefully.")

	return shutdownCtx
}
