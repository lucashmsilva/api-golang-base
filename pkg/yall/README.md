# yall - Yet Another Logging Library

Opinionated Go logging lib, better suited to HTTP servers, that uses Go's `slog` under the hood and provides several quality of life improvements over the standard stdlib logger.

## Main features

* Compatible with any `io.Writer`;
* Compatible with `slog.Attr`;
* Outputs JSON;
* Includes a standalone, performant and production ready `io.Writer` (and `io.Closer`) implementation that outputs to an AWS Firehose stream;
* Includes a default serializer that logs `http.Request`s and supports serializers (see below);
* Supports logging contexts with base attributes that are included in every log record;
* Easy and concurrency safe context switching.

## Table of Contents

- [Logger Usage](#logger-usage)
  - [Creating a new logging instance](#creating-a-new-logging-instance)
  - [Creating a context](#creating-a-context)
  - [Logging](#logging)
  - [Clearing a context](#clearing-a-context)
  - [Cloning a yall logger instance](#cloning-a-yall-logger-instance)
- [AWS Firehose output](#aws-firehose-output)
  - [Usage](#usage)
  - [Options](#options)
  - [Safe cleanup](#safe-cleanup)
- [Serializers](#serializers)
  - [Interface](#interface)
  - [Default serializer](#default-serializer)
  - [Custom serializers](#custom-serializers)

## Logger Usage

`yall` was conceived to be used in HTTP serves and at every incoming request, you should create a new instance, that should be bound to the lifetime of the request. This avoids the need to coordinate and handle concurrent log attribute inclusion or confuse implicit (stdlib) context data handling.

Here's an functional http server that uses a middleware to create a `yall` instance and add it to the incoming request's context.

```go
package logger

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/bermr/api-golang-base/pkg/yall"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func main() {
	r := chi.NewRouter()

	// Logging middleware
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqStartedAt := time.Now()
			lrw := NewLoggingResponseWriter(w)

			// Creates a new yall instance at every request
			logger, err := yall.NewLogger(&yall.LoggerOptions{
				AppName: "app name",
				Version: "v0.0.1",
				Level:   "info",
				Output:  os.Stdout,
			})

			if err != nil {
				slog.Info("logger creation error", "err", err)
				panic(err)
			}

			// Creates a new log context with the request id
			logger.AddLogContext("uuid", uuid.New().String())
			logger.Info("HTTP Request started", r)
			// Outputs: {"time":"2025-06-21T16:30:10.538962886-03:00","level":"INFO","msg":"HTTP Request started","name":"app-name","version":"v0.0.1","hostname":"PC","uuid":"19414659-88af-439b-98fe-7e7632bb6710","req":{"method":"GET","path":"/","ip":"127.0.0.1:45457","user-agent":"curl/7.81.0"}}

			// Saves the logger to the request context for latter use
			loggerContext := context.WithValue(r.Context(), CtxKey("logger"), logger)

			// Registers a callback when the request context is terminated.
			// This pattern is useful to log stuff at the end of the request,
			// such as span time and status code. Also, is ideal, but not mandatory (GC will do the job),
			// to clear the log context when the context information is not needed anymore
			context.AfterFunc(loggerContext, func() {

				// HttpResponseLogData is a custom struct that holds basic HTTP Response data.
				// Can be customized as needed with a new Serializer
				resLogData := &yall.HttpResponseLogData{
					Time:       time.Since(reqStartedAt),
					StatusCode: lrw.statusCode,
					Path:       r.URL.Path,
				}

				logger.Info("HTTP Request finished", resLogData)

				// Clears the log context. Next calls will produce records with only the
				// default base attributes (name, version and hostname)
				logger.ClearLogContext()
			})

			r = r.WithContext(loggerContext)
			next.ServeHTTP(lrw, r)
		})
	})

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		logger, ok := r.Context().Value(CtxKey("logger")).(*yall.Logger)
		if !ok {
			panic(errors.New("no logger set in the request context"))
		}

		logger.Info("important information",
			"custom", "attributes",
		)

		logger.Info("custom attributes from map",
			yall.BuildAttrsFromMap(map[string]any{
				"mapKey":        "value",
				"mixedTypesToo": 123,
			})...)

		logger.Info("standard slog.Attr",
			slog.Attr{
			Key: "standard", 
			Value: slog.AnyValue("slog.Attr"),
		})

		w.Write([]byte("welcome"))
	})

	http.ListenAndServe(":3000", r)
}

type CtxKey string

// Custom http.ResponseWriter implementation that allows logging
// as the server writes a response to the client.
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

func NewLoggingResponseWriter(w http.ResponseWriter) *loggingResponseWriter {
	return &loggingResponseWriter{w, http.StatusOK}
}
```

### Creating a new logging instance

```go
logger, err := yall.NewLogger(&yall.LoggerOptions{
	AppName: "app name",
	Version: "v0.0.1",
	Level:   "info",
	Output:  os.Stdout,
})
```

To create a new yall logger instance, you must provide the `AppName`, it's `Version`, the minimum log `Level` (levels bellow this wil be discarded) and the `Output` stream (must implement `io.Write`). The first three options, along with the hostname, form the base attributes that are included in every record. You can provide a `map[string]any` in the `DefaultAttrs` option. This map will be converted to `[]slog.Attr` and included in every record.

The `Levels` option supports the levels `trace`, `fatal` and `critical` besides the default `slog` standard levels.

The `Serializer` option will be described in it's own section.

### Creating a context

```go
logger.AddLogContext("uuid", uuid.New().String())
```

After creating a instance, you are already ready to start logging, tough is often a good idea to create a new context with some data that you want replicated to all records without the need to explicitly add it. A common use case for this is a request id. In the above example a new logger context is created with the attribute `uuid`. This attribute will be included in all records created after.

Under the hood, yall uses `slog`'s `With()` to create a new instance of the underling logger. This instance is then saved as the default logger for the next calls.

### Logging

```go
logger.Info("HTTP Request started", r)
```
To create a record and send it to the output stream, you must call a method of a greater or equal level of the base logger instance. In above example, the base level is `INFO`, so call of `.Trace()` or `.Debug()` will not produce records and will be dropped.

Yall supports four ways to include additional attributes in a record:
* Standard `slog.Attr`. Single or an array:
```go
logger.Info("standard slog.Attr",
		slog.Attr{
		Key: "standard", 
		Value: slog.AnyValue("slog.Attr"),
	},
)
```
* An `[]any` with key/value pairs:
```go
logger.Info("important information",
	"custom", "attributes",
)
```

* A `map[string]any` of attributes. **In it's current form, only flat maps are supported**. Notice that to have the map properly logged you must use the helper method `yall.BuildAttrsFromMap()` to generate a `[]slog.Attr`.
```go
logger.Info("custom attributes from map",
	yall.BuildAttrsFromMap(map[string]any{
		"mapKey":        "value",
		"mixedTypesToo": 123,
	})...,
)
```

* Structs that have a Serializer that converts them into `slog.Attr`s. Note that to use this feature the `Serializer` option that implements the parsing of the struct must be set during the instantiation. Yall includes a default serializer that parses `error`, `*http.Request` and `yall.*HttpResponseLogData`. To a attribute to be checked for serialization, there must be only one attribute in the list without a key:

```go
logger.Info("HTTP Request started", r) // t.(type) == *http.Request
// ...
resLogData := &yall.HttpResponseLogData{
	Time:       time.Since(reqStartedAt),
	StatusCode: lrw.statusCode,
	Path:       r.URL.Path,
}
logger.Info("HTTP Request finished", resLogData)
//...
logger.Error("HTTP Request finished", errors.New("oops"))
```

### Clearing a context

```go
logger.ClearLogContext()
```

The above call simply overrides the context created with the previous `AddLogContext()`. This is not mandatory as a cleanup step, as it's better to create a new `yall` instance for each incoming request. Clearing the context becomes necessary if you, for some reason, want to log some data for all records to a certain point in the request lifecycle and discard this data after this point. Also, if you want to reuse the same `yall` instance, you need to clear the context. In this last use case, have in mind that concurrent logs might mix the data if there is more than one context.

### Cloning a yall logger instance
If you want to create, and preserve a `yall` logger instance, you can call `yall.GetBaseLogger()` to create a copy of the underling slog.Logger. This is particularly useful when you want other parts of your app that uses `slog` to start generating records with the yall base attributes and to the yall output stream. This could be the case of a gradual migration from `slog` to `yall`:

```go
// Creates a new yall instance at every request
logger, err := yall.NewLogger(&yall.LoggerOptions{
	AppName: "app name",
	Version: "v0.0.1",
	Level:   "info",
	Output:  os.Stdout,
})

if err != nil {
	slog.Info("logger creation error", "err", err)
	panic(err)
}

slog.SetDefault(logger.GetBaseLogger())
```

## AWS Firehose output

As said before `yall` includes a custom `io.Writer` implementation that outputs batches of log records to a AWS Firehose Stream. Typically those streams output to a storage (i.e. S3) and than are ingested in analytics/monitoring solution (i.e. ELK stack).

The implementation allows for total non-blocking (and thread-safe) delivery of logs to the stream and safe cleanup to not lose logs, as it also implements `io.Closer`.

The `FirehoseLogStream` also includes a built in ticker that regularly flushes the buffer even if it never reaches the maximum size.

Finally, it also handles the AWS Firehose service limits of maximum record byte size (1000 KB) and maximin total record batch log size (4 MB)

### Usage

To create and use a new `FirehoseLogStream` follow the next example:

```go
firehoseLogStream, err := yall.NewFirehoseLogStream(&yall.FirehoseLogStreamOptions{
	StreamName: "app-name-stream",
})
if err != nil {
	slog.Info("firehose stream creation error", "err", err)
	panic(err)
}

logger, err := yall.NewLogger(&yall.LoggerOptions{
	AppName: "app name",
	Version: "v0.0.1",
	Level:   "info",
	Output:  firehoseLogStream,
})
```

### Options
To create a new stream you must provide the stream name as it was set in the AWS Firehose configuration. All the other parameters are optional:

* **`MaxBatchSize`**: How many records are buffered before sending to AWS Firehose API. Defaults to 500
* **`WatcherDelay`**: How long the ticker waits before flushing the buffer, even if it never reaches `MaxBatchSize` records. Defaults to 1 second.
* **`Debug`**: Instead of created a AWS Firehose client, it simply sends all records to stdout. Useful if you have not yet configured the AWS Firehose stream.

### Safe cleanup
To avoid losing buffered records when the server is closed, you should store the `FirehoseLogStream` instance and call `stream.Close()` at the server shutdown. This will guarantee that the buffer is flushed. It will be something like this:
```go
firehoseLogStream, err := yall.NewFirehoseLogStream(&yall.FirehoseLogStreamOptions{
	StreamName: "app-name-stream",
})

// All server setup and request handling goes here.

// Setup signal context
rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
defer stop()

// Wait for signal
slog.Info("Listening for process termination signals.")

<-rootCtx.Done()
server.Shutdown(shutdownCtx)
stop()

// safe stream cleanup
firehoseLogStream.Close()
```

## Serializers

As said before,.`yall` has the capability to generate log attributes for some structs using a predefined serialization strategy.

### Interface
A serializer is nothing more than a struct that implements the `Serializer` interface:

```go
type Serializer interface {
	Serialize(any) (slog.Attr, bool)
}
```

The second return value will tell if passed struct is serializable by the current serializer.


### Default serializer
The default serializer has the following implementation to generate attributes to a `http.Request`:

```go
type DefaultSerializers struct{}

func (d *DefaultSerializers) Serialize(attr any) (slog.Attr, bool) {
	switch a := attr.(type) {
	case *http.Request:
		return serializeHttpRequest(a), true
	// ...
	default:
		return slog.Attr{}, false
	}
}

func serializeHttpRequest(r *http.Request) slog.Attr {
	return slog.Group("req",
		slog.Any("method", r.Method),
		slog.Any("path", r.URL.Path),
		slog.Any("ip", r.RemoteAddr),
		slog.Any("user-agent", r.UserAgent()),
	)
}
```

A common pattern is to create a `slog.Group` that will contain the serialized data. This is useful when you are trying to avoid field name collisions in your logging analysis tool.

### Custom serializers

To create a custom serializer you need to implement the interface. If you want to use the default serializers along with this new custom one you are creating you only need to call the `yall.GetDefaultSerializer().Serialize(attr)` method inside your custom one. This will try to serialize `attr` with the default serializer before your custom checks:

```go
package logger

import (
	"log/slog"
	"os"

	"github.com/bermr/api-golang-base/pkg/yall"
)

type MyCustomSerializableStruct struct {
	field string
}

type CustomSerializers struct {
	// Saves the Default Serializer fo latter use
	defaultSerializer yall.Serializer
}

func (c *CustomSerializers) Serialize(attr any) (slog.Attr, bool) {
	// First checks if the Default Serializer can Serialize the attribute.
	// This implementation implies that the Default Serializer has precedence
	// over the new custom one.
	if serializedAttr, ok := c.defaultSerializer.Serialize(attr); ok {
		return serializedAttr, ok
	}

	switch a := attr.(type) {
	case *MyCustomSerializableStruct:
		return serializeMyCustomSerializableStruct(a), true

// You could use the defaultSerializer.Serialize(attr) only for some specific types:
/*
	case *http.Request:
		return defaultSerializer.Serialize(attr)
*/

	default:
		return slog.Attr{}, false
	}
}

func serializeMyCustomSerializableStruct(m *MyCustomSerializableStruct) slog.Attr {
	return slog.Group("customStruct",
		slog.Any("field", m.field),
	)
}

func main() {
	cs := &CustomSerializers{yall.GetDefaultSerializer()}

	logger, err := yall.NewLogger(&yall.LoggerOptions{
		AppName:    "app name",
		Version:    "v0.0.1",
		Level:      "info",
		Output:     os.Stdout,
		Serializer: cs,
	})

	if err != nil {
		slog.Info("logger creation error", "err", err)
		panic(err)
	}

	m := &MyCustomSerializableStruct{"field value"}
	logger.Info("log record with a custom serializable attr", m)
	// Outputs: {"time":"2025-06-21T16:30:10.538962886-03:00","level":"INFO","name":"app-name","version":"v0.0.1","hostname":"PC","msg":"log record with a custom serializable attr","customStruct":{"field": "field value"}}
}
```
