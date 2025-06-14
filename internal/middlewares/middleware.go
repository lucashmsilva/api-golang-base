package middlewares

import (
	"net/http"
)

type MiddlewareHandler interface {
	HandleRequest(http.Handler) http.Handler
}
