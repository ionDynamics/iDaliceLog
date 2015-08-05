package iDaliceLog

import (
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"time"

	"go.iondynamics.net/iDlogger"
	"go.iondynamics.net/iDlogger/priority"
)

// Middleware is a middleware handler that logs the request as it goes in and the response as it goes out.
type Factory struct {
	Logger *iDlogger.Logger

	Priority priority.Priority

	LogPanicsWithPriority priority.Priority
	Stack2Http            bool

}

type Middleware struct {
	*Factory
	Next http.Handler
}

// NewMiddleware returns a new *Middleware, yay!
func NewFactory(log *iDlogger.Logger) *Factory {
	return NewCustomFactory(log, priority.Informational, priority.Emergency, true)
}

func NewCustomFactory(log *iDlogger.Logger, prio, logPanicsWithPriority priority.Priority, stack2http bool) *Factory {
	return &Factory{Logger: log, Priority: prio, LogPanicsWithPriority: logPanicsWithPriority, Stack2Http: stack2http}
}

func (f *Factory) Alice(next http.Handler) http.Handler {
	return &Middleware{Factory: f, Next: next}
}

func (l *Middleware) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := recover(); err != nil {
			stack := make([]byte, 4*1024)
			stack = stack[:runtime.Stack(stack, false)]
			l.Logger.Log(&iDlogger.Event{
				l.Logger,
				map[string]interface{}{
					"panic":       err,
					"status":      http.StatusInternalServerError,
					"method":      r.Method,
					"request":     r.RequestURI,
					"remote":      r.RemoteAddr,
					"text_status": http.StatusText(http.StatusInternalServerError),
					"stack":       string(stack),
				},
				time.Now(),
				l.LogPanicsWithPriority,
				"PANIC while handling request",
			})

			http.Error(rw, "internal server error", http.StatusInternalServerError)

			if l.Stack2Http {
				fmt.Fprintf(rw, "PANIC: %s\n%s", err, stack)
			}

			l.Logger.Wait()
		}
	}()

	start := time.Now()
	startEvent := &iDlogger.Event{
		l.Logger,
		map[string]interface{}{
			"method":  r.Method,
			"request": r.RequestURI,
			"remote":  r.RemoteAddr,
		},
		time.Now(),
		l.Priority,
		"started handling request",
	}
	l.Logger.Log(startEvent)

	lrw := &LogResponseWriter{ResponseWriter: rw}

	l.Next.ServeHTTP(lrw, r)

	latency := time.Since(start)

	completedEvent := &iDlogger.Event{
		l.Logger,
		map[string]interface{}{
			"status":      strconv.Itoa(lrw.Status()),
			"method":      r.Method,
			"request":     r.RequestURI,
			"remote":      r.RemoteAddr,
			"text_status": http.StatusText(lrw.Status()),
			"took":        latency.String(),
			"size":        lrw.Size(),
		},
		time.Now(),
		l.Priority,
		"completed handling request",
	}
	l.Logger.Log(completedEvent)
}
