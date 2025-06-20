package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"

	"github.com/bermr/api-golang-base/internal/config"
	"github.com/go-chi/chi/v5"
)

type Server struct {
	server         *http.Server
	requestStopper context.CancelFunc
	config         *config.Config
}

func New(c *config.Config, r *chi.Mux) *Server {
	ongoingCtx, requestStopper := context.WithCancel(context.Background())
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%v", c.Port),
		Handler: r,
		BaseContext: func(_ net.Listener) context.Context {
			return ongoingCtx
		},
	}

	return &Server{srv, requestStopper, c}
}

func (s *Server) Start() error {
	slog.Info("server started", "port", s.config.Port)
	return s.server.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}
