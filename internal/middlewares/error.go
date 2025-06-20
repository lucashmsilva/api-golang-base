package middlewares

import (
	"errors"
	"net/http"

	"github.com/bermr/api-golang-base/internal/tools/util"
	"github.com/bermr/api-golang-base/pkg/yall"
)

type ErrorMiddleware struct{}

func NewErrorMiddleware() *ErrorMiddleware {
	return &ErrorMiddleware{}
}

func (em *ErrorMiddleware) HandleRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				logger, ok := r.Context().Value(util.CtxKey("_reqLogger")).(*yall.Logger)
				if !ok {
					panic(errors.New("no logger set in base context"))
				}

				logger.Info("info", "HTTP Request error", err)
				http.Error(w, "Oops! Something went wrong.", http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	})
}
