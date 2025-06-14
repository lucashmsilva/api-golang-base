package middlewares

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/bermr/api-golang-base/internal/config"
	"github.com/bermr/api-golang-base/internal/tools/logger"
	"github.com/bermr/api-golang-base/internal/tools/my_logger"
	"github.com/bermr/api-golang-base/internal/tools/util"
	"github.com/google/uuid"
)

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

type RequestLoggerMiddleware struct {
	config             *config.Config
	loggerOutputStream io.Writer
}

func NewLoggingResponseWriter(w http.ResponseWriter) *loggingResponseWriter {
	return &loggingResponseWriter{w, http.StatusOK}
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

func NewLoggerMiddleware(config *config.Config, loggerOutputStream io.Writer) *RequestLoggerMiddleware {
	return &RequestLoggerMiddleware{config, loggerOutputStream}
}

func (lm RequestLoggerMiddleware) HandleRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqStartedAt := time.Now()
		lrw := NewLoggingResponseWriter(w)
		log := logger.GetLogger(lm.config, lm.loggerOutputStream)

		log.AddLogContext("uuid", uuid.New().String())
		log.Info("HTTP Request started", r)

		loggerContext := context.WithValue(r.Context(), util.CtxKey("_reqLogger"), log)
		context.AfterFunc(loggerContext, func() {
			resLogData := &my_logger.HttpResponseLogData{
				Time:       time.Since(reqStartedAt),
				StatusCode: lrw.statusCode,
				Path:       r.URL.Path,
			}

			log.Info("HTTP Request finished", resLogData)
			log.ClearLogContext()
		})

		r = r.WithContext(loggerContext)
		next.ServeHTTP(lrw, r)
	})
}
