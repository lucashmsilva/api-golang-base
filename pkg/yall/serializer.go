package yall

import (
	"log/slog"
	"net/http"
	"time"
)

type Serializer interface {
	// Takes a concrete struct, checks is it serializable and, if it is, returns a slog.Attr
	// with the parsing result along with true, otherwise, returns a zero slog.Attr and false
	Serialize(any) (slog.Attr, bool)
}

type DefaultSerializers struct{}

// Helper struct to log basic information about a HTTP response
type HttpResponseLogData struct {
	Time       time.Duration
	StatusCode int
	Path       string
}

// Parses error, *http.Request and yall.*HttpResponseLogData to slog.Group
func (d *DefaultSerializers) Serialize(attr any) (slog.Attr, bool) {
	switch a := attr.(type) {
	case error:
		return serializeError(a), true

	case *http.Request:
		return serializeHttpRequest(a), true

	case *HttpResponseLogData:
		return serializeHttpResponse(a), true

	default:
		return slog.Attr{}, false
	}
}

func serializeError(e error) slog.Attr {
	return slog.Group("err", slog.Any("msg", e.Error()))
}

func serializeHttpRequest(r *http.Request) slog.Attr {
	return slog.Group("req",
		slog.Any("method", r.Method),
		slog.Any("path", r.URL.Path),
		slog.Any("ip", r.RemoteAddr),
		slog.Any("user-agent", r.UserAgent()),
	)
}

func serializeHttpResponse(r *HttpResponseLogData) slog.Attr {
	return slog.Group("res",
		slog.Any("status", r.StatusCode),
		slog.Any("path", r.Path),
		slog.Any("time", r.Time.String()),
	)
}

// Returns a copy of the DefaultSerializers implementation
func GetDefaultSerializer() Serializer {
	return &DefaultSerializers{}
}
