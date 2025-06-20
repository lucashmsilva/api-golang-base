package middlewares

import (
	"errors"
	"net/http"

	"github.com/bermr/api-golang-base/internal/tools/my_logger"
	"github.com/bermr/api-golang-base/internal/tools/util"
)

type ErrorMiddleware struct{}

func NewErrorMiddleware() *ErrorMiddleware {
	return &ErrorMiddleware{}
}

func (em *ErrorMiddleware) HandleRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				logger, ok := r.Context().Value(util.CtxKey("_reqLogger")).(*my_logger.Logger)
				if !ok {
					panic(errors.New("no logger set in base context"))
				}

				logger.Log(r.Context(), "info", "HTTP Request error", err)
				http.Error(w, "Oops! Something went wrong.", http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	})
}
